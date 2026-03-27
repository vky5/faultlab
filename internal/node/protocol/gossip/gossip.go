package gossip

import (
	"math/rand"

	"github.com/vky5/faultlab/internal/node/protocol"
)

type GossipProtocol struct {
	nodeID string

	tick uint64 // logical time

	peers []string

	store map[string]Value

	gossipInterval uint64 // after n ticks -> send digest to random peer
}

func NewGossipProtocol() *GossipProtocol {
	return &GossipProtocol{
		nodeID:         "",
		peers:          []string{},
		store:          make(map[string]Value),
		gossipInterval: 3,
	}
}

// initialize the gossip protocol, set the nodeID and start the logical clock
func (g *GossipProtocol) Start(nodeID string) error {
	g.nodeID = nodeID
	g.tick = 0
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
		return []protocol.Envelope{}
	}

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
	msg, err := protocol.DecodeJSON[GossipMessage](env.Payload)
	if err != nil {
		return []protocol.Envelope{}
	}

	switch msg.Type {
	case MsgDigest:
		return g.handleDigest(env.From, msg)
	case MsgState:
		return g.handleState(msg)
	default:
		return []protocol.Envelope{}
	}
}

// when we receive the digest message, we compare the versions and prepare the state message in which we (the current Node) has some updated value and send it back to the sender
func (g *GossipProtocol) handleDigest(from string, msg GossipMessage) []protocol.Envelope {
	// compare the versions and prepare the state message in which we (the current Node) has some updated value
	stateMsg := GossipMessage{
		Type:  MsgState,
		State: make(map[string]Value),
	}

	// iterate over LOCAL store (not remote digest)
	for key, localVal := range g.store {
		remoteVer, ok := msg.Digest[key]

		// if remote doesn't have the key OR is outdated
		if !ok || localVal.Version > remoteVer {
			stateMsg.State[key] = localVal
		}
	}

	// nothing to send
	if len(stateMsg.State) == 0 {
		return nil
	}

	envelope := protocol.Envelope{
		From:        g.nodeID,
		To:          from,
		Protocol:    "gossip",
		Payload:     protocol.EncodeJSON(stateMsg),
		Kind:        protocol.KindProtocol,
		Version:     1,
		LogicalTick: g.tick,
	}

	return []protocol.Envelope{envelope}
}

// when we receive the state message, we update our local store with the received state
func (g *GossipProtocol) handleState(msg GossipMessage) []protocol.Envelope {
	// update the local store with the received state
	for k, v := range msg.State {
		if localVal, ok := g.store[k]; !ok || localVal.Version < v.Version {
			g.store[k] = v
		}
	}
	return []protocol.Envelope{} // Left it we ennable PUSH PULL later
}

// for debugging and testing purposes
func (g *GossipProtocol) State() any {
	return map[string]any{
		"store": g.store,
	}
}

func (g *GossipProtocol) Stop() error {
	return nil
}

// (peer, ok) if no peers available, ok = false
func (g *GossipProtocol) pickRandomPeer() (string, bool) {
	if len(g.peers) == 0 {
		return "", false
	}
	return g.peers[rand.Intn(len(g.peers))], true // right now we are fanning out to single peers but we can send a list
}

func init() {
	protocol.Register("gossip", func() protocol.ClusterProtocol {
		return NewGossipProtocol()
	})
}
