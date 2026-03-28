package gossip

import (
	"log"
	"math/rand"
	"strconv"
	"strings"

	"github.com/vky5/faultlab/internal/node/protocol"
)

const (
	gossipColorCyan  = "\033[36m"
	gossipColorReset = "\033[0m"
)

func logGossipf(format string, args ...any) {
	log.Printf(gossipColorCyan+format+gossipColorReset, args...)
}

type GossipProtocol struct {
	nodeID string

	tick uint64 // logical time

	peers []string

	store map[string]Value

	gossipInterval uint64 // after n ticks -> send digest to random peer

	// Callback for dynamic peer discovery.
	peerDiscoveryCb protocol.PeerDiscoveryCallback
}

func NewGossipProtocol() *GossipProtocol {
	logGossipf("[gossip] NewGossipProtocol created")
	return &GossipProtocol{
		nodeID:         "",
		peers:          []string{},
		store:          make(map[string]Value),
		gossipInterval: 3,
	}
}

// initialize the gossip protocol, set the nodeID and start the logical clock
func (g *GossipProtocol) Start(nodeID string) error {
	logGossipf("[gossip] Start called for node %s", nodeID)
	g.nodeID = nodeID
	g.tick = 0
	return nil
}

// the tick the main GOAT func
func (g *GossipProtocol) Tick() []protocol.Envelope {
	g.tick++

	// Print current store state
	if len(g.store) > 0 {
		logGossipf("[gossip] node %s tick %d - STORE CONTENTS:", g.nodeID, g.tick)
		for k, v := range g.store {
			logGossipf("[gossip]   %s = {Data: %q, Version: %d}", k, v.Data, v.Version)
		}
	} else {
		logGossipf("[gossip] node %s tick %d - STORE is empty", g.nodeID, g.tick)
	}

	if g.tick%g.gossipInterval != 0 {
		return nil
	}

	gm := GossipMessage{
		Type:   MsgDigest,
		Digest: make(map[string]int64),
	}
	for k, v := range g.store {
		gm.Digest[k] = v.Version // {a: 5, b: 2} not actual value
	}

	// pick a random peer and send the digest message
	peer, ok := g.pickRandomPeer()
	if !ok {
		logGossipf("[gossip] node %s tick %d: no peers to gossip with", g.nodeID, g.tick)
		return []protocol.Envelope{}
	}

	logGossipf("[gossip] node %s tick %d: sending digest with %d keys to %s", g.nodeID, g.tick, len(gm.Digest), peer)

	envelope := protocol.Envelope{
		From:        g.nodeID,
		To:          peer,
		Protocol:    "gossip",
		Payload:     protocol.EncodeJSON(gm),
		Kind:        protocol.KindProtocol,
		Version:     1,
		LogicalTick: g.tick,
	}
	return []protocol.Envelope{envelope}
}

func (g *GossipProtocol) OnMessage(env protocol.Envelope) []protocol.Envelope {
	senderID, senderHost, senderPort := parseSenderAddress(env.From)
	if senderID == "" {
		senderID = env.From
	}

	if senderID != "" && senderID != g.nodeID && !g.hasPeer(senderID) {
		g.peers = append(g.peers, senderID)
		logGossipf("[gossip] discovered new peer %s from incoming message", senderID)

		if g.peerDiscoveryCb != nil && senderHost != "" && senderPort > 0 {
			g.peerDiscoveryCb.OnPeerDiscovered(senderID, senderHost, senderPort)
		}
	}

	logGossipf("[gossip] node %s received %d bytes from %s at tick %d", g.nodeID, len(env.Payload), env.From, g.tick)
	msg, err := protocol.DecodeJSON[GossipMessage](env.Payload)
	if err != nil {
		logGossipf("[gossip] decode failed from %s: %v", env.From, err)
		return []protocol.Envelope{}
	}

	switch msg.Type {
	case MsgDigest:
		logGossipf("[gossip] handling DIGEST from %s (%d keys)", senderID, len(msg.Digest))
		return g.handleDigest(senderID, msg)
	case MsgState:
		logGossipf("[gossip] handling STATE from %s (%d keys)", senderID, len(msg.State))
		return g.handleState(msg)
	default:
		logGossipf("[gossip] unknown message type %q from %s", msg.Type, senderID)
		return []protocol.Envelope{}
	}
}

// when we receive the digest message, we compare the versions and prepare the state message in which we (the current Node) has some updated value and send it back to the sender
// compare the versions and prepare the state message in which we (the current Node) has some updated value
func (g *GossipProtocol) handleDigest(from string, msg GossipMessage) []protocol.Envelope {
	var out []protocol.Envelope

	// ---------- 1. SEND STATE (push fix for them) ----------
	stateMsg := GossipMessage{
		Type:  MsgState,
		State: make(map[string]Value),
	}

	for key, localVal := range g.store {
		remoteVer, ok := msg.Digest[key]
		if !ok || localVal.Version > remoteVer {
			stateMsg.State[key] = localVal
		}
	}

	if len(stateMsg.State) > 0 {
		logGossipf("[gossip] sending STATE to %s with %d updated keys", from, len(stateMsg.State))
		out = append(out, protocol.Envelope{
			From:        g.nodeID,
			To:          from,
			Protocol:    "gossip",
			Payload:     protocol.EncodeJSON(stateMsg),
			Kind:        protocol.KindProtocol,
			Version:     1,
			LogicalTick: g.tick,
		})
	}

	// ---------- 2. CHECK IF I AM BEHIND ----------
	behind := false
	for key, remoteVer := range msg.Digest {
		if localVal, ok := g.store[key]; !ok || localVal.Version < remoteVer {
			behind = true
			break
		}
	}

	// ---------- 3. SEND DIGEST BACK (trigger reverse sync) ----------
	if behind {
		logGossipf("[gossip] local node %s is behind %s, requesting reverse sync", g.nodeID, from)
		digest := make(map[string]int64)
		for k, v := range g.store {
			digest[k] = v.Version
		}

		digestMsg := GossipMessage{
			Type:   MsgDigest,
			Digest: digest,
		}

		out = append(out, protocol.Envelope{
			From:        g.nodeID,
			To:          from,
			Protocol:    "gossip",
			Payload:     protocol.EncodeJSON(digestMsg),
			Kind:        protocol.KindProtocol,
			Version:     1,
			LogicalTick: g.tick,
		})
	}

	if len(out) == 0 {
		logGossipf("[gossip] no response needed for digest from %s", from)
		return nil
	}

	logGossipf("[gossip] prepared %d outbound message(s) for %s", len(out), from)

	return out
}

// for setting up peers
func (g *GossipProtocol) SetPeers(peers []string) {
	g.peers = make([]string, len(peers))
	copy(g.peers, peers)
	logGossipf("[gossip] SetPeers called for node %s: %v", g.nodeID, g.peers)
}

// SetPeerDiscoveryCallback sets the callback for dynamic peer discovery.
func (g *GossipProtocol) SetPeerDiscoveryCallback(cb protocol.PeerDiscoveryCallback) {
	g.peerDiscoveryCb = cb
}

// when we receive the state message, we update our local store with the received state
func (g *GossipProtocol) handleState(msg GossipMessage) []protocol.Envelope {
	updated := 0
	// update the local store with the received state
	for k, v := range msg.State {
		if localVal, ok := g.store[k]; !ok || localVal.Version < v.Version {
			g.store[k] = v
			updated++
		}
	}
	logGossipf("[gossip] applied STATE update: %d/%d keys changed", updated, len(msg.State))
	return []protocol.Envelope{} // Left it we ennable PUSH PULL later
}

// for debugging and testing purposes
func (g *GossipProtocol) State() any {
	return map[string]any{
		"store": g.store,
	}
}

func (g *GossipProtocol) Stop() error {
	logGossipf("[gossip] Stop called for node %s", g.nodeID)
	return nil
}

// (peer, ok) if no peers available, ok = false
func (g *GossipProtocol) pickRandomPeer() (string, bool) {
	if len(g.peers) == 0 {
		return "", false
	}
	peer := g.peers[rand.Intn(len(g.peers))]
	logGossipf("[gossip] node %s: selected peer %s from %d available peers", g.nodeID, peer, len(g.peers))
	return peer, true // right now we are fanning out to single peers but we can send a list
}

func (g *GossipProtocol) hasPeer(peerID string) bool {
	for _, p := range g.peers {
		if p == peerID {
			return true
		}
	}
	return false
}

func parseSenderAddress(from string) (peerID, host string, port int) {
	peerID = from

	idx := strings.Index(from, "@")
	if idx == -1 {
		return peerID, "", 0
	}

	peerID = from[:idx]
	addrStr := from[idx+1:]
	colonIdx := strings.LastIndex(addrStr, ":")
	if colonIdx == -1 {
		return peerID, addrStr, 0
	}

	host = addrStr[:colonIdx]
	parsedPort, err := strconv.Atoi(addrStr[colonIdx+1:])
	if err != nil {
		return peerID, host, 0
	}

	return peerID, host, parsedPort
}

// PUT is api to update the local store
func (g *GossipProtocol) Put(key, data string) {
	current, ok := g.store[key]
	if !ok {
		g.store[key] = Value{Data: data, Version: 1}
		logGossipf("[gossip] PUT created key=%s version=%d", key, g.store[key].Version)
	} else {
		g.store[key] = Value{Data: data, Version: current.Version + 1}
		logGossipf("[gossip] PUT updated key=%s version=%d", key, g.store[key].Version)
	}
}

func init() {
	protocol.Register("gossip", func() protocol.ClusterProtocol {
		return NewGossipProtocol()
	})
}

/*
What will happen if in current implementation we have
init x - A v1 in node 1
and init x - B v1 in node 2

- A sends digest to B with x:1
- B sends digest to A with x:1
- A sends state to B with x:1 (because it thinks B is behind)
- B sends state to A with x:1 (because it thinks A is behind)

Now we have two different value for same key with same version, this is called conflict and we need some conflict resolution strategy to resolve it, otherwise we will end up in infinite loop of sending digest and state back and forth.
*/
