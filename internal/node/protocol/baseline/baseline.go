package baseline

import (
	"log"

	"github.com/vky5/faultlab/internal/node/protocol"
)

type NodeStatus int

const (
	StatusAlive NodeStatus = iota
	StatusSuspect
	StatusDead
)

// defines the basic physics (state) of the cluster
type BaselineProtocol struct {
	nodeID string

	tick uint64 // logical time

	heartbeatInterval uint64 // after how many logical ticks heartbeat to send to
	timeoutTicks      uint64

	lastSeen     map[string]uint64
	status       map[string]NodeStatus
	suspectSince map[string]uint64

	peers []string
}

// creating a baseline
func NewBaselineProtocol(peers []string) *BaselineProtocol {
	log.Printf("[baseline] NewBaselineProtocol created with %d peers\n", len(peers))
	return &BaselineProtocol{
		heartbeatInterval: 5,
		timeoutTicks:      20,
		lastSeen:          make(map[string]uint64),
		status:            make(map[string]NodeStatus),
		suspectSince:      make(map[string]uint64),
		peers:             peers,
	}
}

// set the initial state of the node
func (b *BaselineProtocol) Start(nodeID string) error {
	log.Printf("[baseline] Start called for node %s\n", nodeID)
	b.nodeID = nodeID
	b.tick = 0

	for _, p := range b.peers {
		b.lastSeen[p] = 0
		b.status[p] = StatusAlive
	}
	log.Printf("[baseline] Node %s initialized with peers: %v\n", nodeID, b.peers)
	return nil
}

func (b *BaselineProtocol) Tick() []protocol.Envelope {
	b.tick++

	var out []protocol.Envelope

	// periodic heartbeat
	if b.tick%b.heartbeatInterval == 0 {
		log.Printf("[baseline] Node %s tick %d: sending heartbeats to %d peers\n", b.nodeID, b.tick, len(b.peers))
		for _, peer := range b.peers {
			env := protocol.Envelope{
				From:     b.nodeID,
				To:       peer,
				Protocol: "baseline",
				Kind:     protocol.KindProtocol,
				Payload:  []byte("HEARTBEAT"),
			}

			out = append(out, env)
		}
	}

	for _, peer := range b.peers {
		last := b.lastSeen[peer]
		diff := b.tick - last

		switch b.status[peer] {
		case StatusAlive:
			if diff > b.timeoutTicks {
				b.status[peer] = StatusSuspect
				b.suspectSince[peer] = b.tick
				log.Printf("[baseline] %s SUSPECT at tick %d", peer, b.tick)
				// SUSPECT is not being broadcasted because it is uncertain 
			}

		case StatusSuspect:
			if b.tick-b.suspectSince[peer] > b.timeoutTicks {
				b.status[peer] = StatusDead
				log.Printf("[baseline] %s DEAD at tick %d", peer, b.tick)

				// DEAD being broadcasted because we are sure that this is the truth
				out = append(out, protocol.Envelope{
					From:     b.nodeID,
					To:       "", // BROADCAST entirety of it in a cluster
					Protocol: "baseline",
					Kind:     protocol.KindProtocol,
					Payload:  []byte("DEAD" + peer),
				})
			}
		}

	}

	return out
}

func (b *BaselineProtocol) OnMessage(env protocol.Envelope) []protocol.Envelope {
	log.Printf("[baseline] Node %s received message from %s at tick %d\n", b.nodeID, env.From, b.tick)

	// update liveness info
	b.lastSeen[env.From] = b.tick //* At any logical time(tick) I heard from this

	if b.status[env.From] != StatusAlive {
		log.Printf("[baseline] %s revived", env.From)
		b.status[env.From] = StatusAlive
	}

	// dead gossip merge
	if string(env.Payload[:4]) == "DEAD" {
		deadNode := string(env.Payload[5:])
		b.status[deadNode] = StatusDead
	}

	// baseline currently does not respond
	return nil
}

func (b *BaselineProtocol) Stop() error {
	log.Printf("[baseline] Stop called for node %s\n", b.nodeID)
	return nil
}

// SetPeers updates the peer list dynamically
func (b *BaselineProtocol) SetPeers(peers []string) {
	log.Printf("[baseline] SetPeers called: %v\n", peers)
	b.peers = make([]string, len(peers))
	copy(b.peers, peers)

	// Initialize lastSeen for new peers
	for _, p := range peers {
		if _, exists := b.lastSeen[p]; !exists {
			b.lastSeen[p] = 0
		}
	}
}

func (b *BaselineProtocol) State() any {
	return map[string]any{
		"node":      b.nodeID,
		"tick":      b.tick,
		"last_seen": b.lastSeen,
	}
}

/*
?two nodes in a cluster will not have same time. This is the difference between logical time and real time
?
*/

func init() { // init runs anytime someone imports the module
	log.Println("[baseline] init: registering baseline protocol")
	protocol.Register("baseline", func() protocol.ClusterProtocol {
		return NewBaselineProtocol(nil) // this actually returns object of type Baselineprotocol on which we perform operation we store this struct and since this struct impleemnts all the func of the interface it fits perfectly
	})
}
