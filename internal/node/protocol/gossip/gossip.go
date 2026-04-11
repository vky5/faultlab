package gossip

import (
	"fmt"
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

func (g *GossipProtocol) logGossipf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	log.Printf(gossipColorCyan+"[gossip] %s"+gossipColorReset, msg)
	if g.logger != nil {
		if l, ok := g.logger.(interface{ Printf(string, ...any) }); ok {
			l.Printf(msg)
		}
	}
}

type GossipProtocol struct {
	nodeID string

	tick uint64 // logical time

	peers []string

	store map[string]Value

	gossipInterval uint64 // after n ticks -> send digest to random peer

	// Callback for dynamic peer discovery.
	peerDiscoveryCb protocol.PeerDiscoveryCallback

	logger any
}

func NewGossipProtocol() *GossipProtocol {
	g := &GossipProtocol{
		nodeID:         "",
		peers:          []string{},
		store:          make(map[string]Value),
		gossipInterval: 3,
	}
	g.logGossipf("NewGossipProtocol created")
	return g
}

// initialize the gossip protocol, set the nodeID and start the logical clock
func (g *GossipProtocol) Start(nodeID string) error {
	g.nodeID = nodeID
	g.tick = 0
	g.logGossipf("Start called for node %s", nodeID)
	return nil
}

// the tick the main GOAT func
func (g *GossipProtocol) Tick() []protocol.Envelope {
	g.tick++

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
		g.logGossipf("node %s tick %d: no peers to gossip with", g.nodeID, g.tick)
		return nil
	}

	// Create key summary for metadata
	keys := make([]string, 0, len(gm.Digest))
	for k := range gm.Digest {
		keys = append(keys, k)
	}
	keyPrefix := "keys=["
	if len(keys) > 0 {
		keyPrefix += strings.Join(keys, ",")
	}
	keyPrefix += "]"
	if len(keyPrefix) > 100 {
		keyPrefix = keyPrefix[:97] + "..."
	}

	payload := protocol.EncodeJSON(gm)
	g.logGossipf("node %s tick %d: sending digest with %d keys to %s (%d bytes)", g.nodeID, g.tick, len(gm.Digest), peer, len(payload))

	envelope := protocol.Envelope{
		From:          g.nodeID,
		To:            peer,
		Protocol:      "gossip",
		Payload:       payload,
		Kind:          protocol.KindProtocol,
		TraceMetadata: fmt.Sprintf("GOSSIP_DIGEST:%s", keyPrefix),
		Version:       1,
		LogicalTick:   g.tick,
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
		g.logGossipf("discovered new peer %s from incoming message", senderID)

		if g.peerDiscoveryCb != nil && senderHost != "" && senderPort > 0 {
			g.peerDiscoveryCb.OnPeerDiscovered(senderID, senderHost, senderPort)
		}
	}

	g.logGossipf("node %s received %d bytes from %s at tick %d", g.nodeID, len(env.Payload), env.From, g.tick)
	msg, err := protocol.DecodeJSON[GossipMessage](env.Payload)
	if err != nil {
		g.logGossipf("decode failed from %s: %v", env.From, err)
		return nil
	}

	switch msg.Type {
	case MsgDigest:
		g.logGossipf("handling DIGEST from %s (%d keys)", senderID, len(msg.Digest))
		return g.handleDigest(senderID, msg)
	case MsgState:
		g.logGossipf("handling STATE from %s (%d keys)", senderID, len(msg.State))
		return g.handleState(msg)
	default:
		g.logGossipf("unknown message type %q from %s", msg.Type, senderID)
		return nil
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
		// Create state summary
		updates := make([]string, 0, len(stateMsg.State))
		for k, v := range stateMsg.State {
			updates = append(updates, fmt.Sprintf("%s=%v", k, v.Data))
		}
		stateMeta := "updates={" + strings.Join(updates, ",") + "}"
		if len(stateMeta) > 100 {
			stateMeta = stateMeta[:97] + "..."
		}

		payload := protocol.EncodeJSON(stateMsg)
		g.logGossipf("sending STATE to %s with %d updated keys (%d bytes)", from, len(stateMsg.State), len(payload))
		out = append(out, protocol.Envelope{
			From:          g.nodeID,
			To:            from,
			Protocol:      "gossip",
			Payload:       payload,
			Kind:          protocol.KindProtocol,
			TraceMetadata: fmt.Sprintf("GOSSIP_STATE:%s", stateMeta),
			Version:       1,
			LogicalTick:   g.tick,
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
		g.logGossipf("local node %s is behind %s, requesting reverse sync", g.nodeID, from)

		digestMsg := GossipMessage{
			Type:   MsgDigest,
			Digest: make(map[string]int64),
		}
		for k, v := range g.store {
			digestMsg.Digest[k] = v.Version
		}

		payload := protocol.EncodeJSON(digestMsg)

		out = append(out, protocol.Envelope{
			From:          g.nodeID,
			To:            from,
			Protocol:      "gossip",
			Payload:       payload,
			Kind:          protocol.KindProtocol,
			TraceMetadata: fmt.Sprintf("GOSSIP_SYNC_REQ:%d_keys", len(digestMsg.Digest)),
			Version:       1,
			LogicalTick:   g.tick,
		})
	}

	if len(out) == 0 {
		g.logGossipf("no response needed for digest from %s", from)
		return nil
	}

	g.logGossipf("prepared %d outbound message(s) for %s", len(out), from)

	return out
}

// for setting up peers
func (g *GossipProtocol) SetPeers(peers []string) {
	filtered := make([]string, 0, len(peers))
	for _, p := range peers {
		if p != "" && p != g.nodeID {
			filtered = append(filtered, p)
		}
	}
	g.peers = filtered
	g.logGossipf("SetPeers called for node %s: %v", g.nodeID, g.peers)
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
		if localVal, ok := g.store[k]; !ok || isIncomingNewer(localVal, v) {
			g.store[k] = v
			updated++
		}
	}
	g.logGossipf("applied STATE update: %d/%d keys changed", updated, len(msg.State))
	return nil // Left it we ennable PUSH PULL later
}

// for debugging and testing purposes
func (g *GossipProtocol) State() any {
	return map[string]any{
		"store": g.store,
	}
}

func (g *GossipProtocol) Stop() error {
	g.logGossipf("Stop called for node %s", g.nodeID)
	return nil
}

// (peer, ok) if no peers available, ok = false
func (g *GossipProtocol) pickRandomPeer() (string, bool) {
	if len(g.peers) == 0 {
		return "", false
	}
	peer := g.peers[rand.Intn(len(g.peers))]
	g.logGossipf("node %s: selected peer %s from %d available peers", g.nodeID, peer, len(g.peers))
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
		g.store[key] = Value{Data: data, Version: 1, NodeID: g.nodeID}
		g.logGossipf("PUT created key=%s version=%d", key, g.store[key].Version)
	} else {
		g.store[key] = Value{Data: data, Version: current.Version + 1, NodeID: g.nodeID}
		g.logGossipf("PUT updated key=%s version=%d", key, g.store[key].Version)
	}
}

// Get retrieves a value from the local store
func (g *GossipProtocol) Get(key string) (string, bool) {
	val, ok := g.store[key]
	if !ok {
		return "", false
	}
	return val.Data, true
}

func (g *GossipProtocol) GetWithMetadata(key string) (string, int64, string, bool) {
	val, ok := g.store[key]
	if !ok {
		return "", 0, "", false
	}
	return val.Data, val.Version, val.NodeID, true
}

// Delete removes a key from the local store (version-neutral for now)
func (g *GossipProtocol) Delete(key string) {
	if _, ok := g.store[key]; ok {
		delete(g.store, key)
		g.logGossipf("DELETE key=%s", key)
	}
}

// comparing the digest
func isIncomingNewer(local, incoming Value) bool {
	if incoming.Version > local.Version {
		return true
	}
	if incoming.Version < local.Version {
		return false
	}
	// tie-break
	return incoming.NodeID > local.NodeID
}

func (g *GossipProtocol) SetLogger(l any) {
	g.logger = l
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
