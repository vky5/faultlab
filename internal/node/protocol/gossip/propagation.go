package gossip

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/vky5/faultlab/internal/node/protocol"
)

// Tick advances the gossip logical clock and periodically sends digest metadata.
func (g *GossipProtocol) Tick() []protocol.Envelope {
	g.tick++

	if g.tick%g.gossipInterval != 0 {
		return nil
	}

	gm := GossipMessage{
		Type:   MsgDigest,
		Digest: make(map[string]DigestEntry),
	}
	for k, v := range g.store {
		gm.Digest[k] = valueToDigestEntry(v)
	}

	var peer string
	var ok bool

	if len(g.forceSyncPeers) > 0 {
		for p := range g.forceSyncPeers {
			peer = p
			ok = true
			delete(g.forceSyncPeers, p)
			g.logGossipf("node %s tick %d: ACTIVE SYNC to new peer %s (forceSyncPeers remaining: %d)", g.nodeID, g.tick, peer, len(g.forceSyncPeers))
			break
		}
	} else {
		peer, ok = g.pickRandomPeer()
	}

	if !ok {
		g.logGossipf("node %s tick %d: no peers to gossip with", g.nodeID, g.tick)
		return nil
	}

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
		LogicalTick:   uint64(time.Now().UnixMilli()),
	}
	return []protocol.Envelope{envelope}
}

// OnMessage routes inbound gossip envelopes by message type and handles peer discovery.
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

func (g *GossipProtocol) handleDigest(from string, msg GossipMessage) []protocol.Envelope {
	var out []protocol.Envelope

	stateMsg := GossipMessage{
		Type:  MsgState,
		State: make(map[string]Value),
	}

	for key, localVal := range g.store {
		remoteEntry, ok := msg.Digest[key]
		if !ok || compareValueToDigest(localVal, remoteEntry) > 0 {
			stateMsg.State[key] = localVal
		}
	}

	if len(stateMsg.State) > 0 {
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
			LogicalTick:   uint64(time.Now().UnixMilli()),
		})
	}

	behind := false
	for key, remoteEntry := range msg.Digest {
		if localVal, ok := g.store[key]; !ok || compareValueToDigest(localVal, remoteEntry) < 0 {
			behind = true
			break
		}
	}

	if behind {
		g.logGossipf("local node %s is behind %s, requesting reverse sync", g.nodeID, from)

		digestMsg := GossipMessage{
			Type:   MsgDigest,
			Digest: make(map[string]DigestEntry),
		}
		for k, v := range g.store {
			digestMsg.Digest[k] = valueToDigestEntry(v)
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
			LogicalTick:   uint64(time.Now().UnixMilli()),
		})
	}

	if len(out) == 0 {
		g.logGossipf("no response needed for digest from %s", from)
		return nil
	}

	g.logGossipf("prepared %d outbound message(s) for %s", len(out), from)

	return out
}

// (peer, ok) if no peers available, ok = false
func (g *GossipProtocol) pickRandomPeer() (string, bool) {
	if len(g.peers) == 0 {
		return "", false
	}
	peer := g.peers[rand.Intn(len(g.peers))]
	g.logGossipf("node %s: selected peer %s from %d available peers", g.nodeID, peer, len(g.peers))
	return peer, true
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

func valueToDigestEntry(v Value) DigestEntry {
	vcCopy := make(map[string]int64)
	if v.VectorClock != nil {
		for k, val := range v.VectorClock {
			vcCopy[k] = val
		}
	}
	return DigestEntry{
		Version:     v.Version,
		Origin:      v.Origin,
		Timestamp:   v.Timestamp,
		VectorClock: vcCopy,
	}
}

func compareValueToDigest(local Value, remote DigestEntry) int {
	vcCmp := vectorClockCompare(local.VectorClock, remote.VectorClock)
	if vcCmp != 0 {
		return vcCmp
	}

	if local.Timestamp > remote.Timestamp {
		return 1
	}
	if local.Timestamp < remote.Timestamp {
		return -1
	}
	if local.Origin > remote.Origin {
		return 1
	}
	if local.Origin < remote.Origin {
		return -1
	}
	if local.Version > remote.Version {
		return 1
	}
	if local.Version < remote.Version {
		return -1
	}
	return 0
}
