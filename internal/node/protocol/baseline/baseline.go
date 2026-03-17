package baseline

import "github.com/vky5/faultlab/internal/node/protocol"

// defines the basic physics (state) of the cluster
type BaselineProtocol struct {
	nodeID string

	tick uint64 // logical time

	heartbeatInterval uint64 // after how many logical ticks heartbeat to send to
	timeoutTicks      uint64

	lastSeen map[string]uint64
	peers    []string
}

// creating a baseline
func NewBaselineProtocol(peers []string) *BaselineProtocol {
	return &BaselineProtocol{
		heartbeatInterval: 5,
		timeoutTicks:      20,
		lastSeen:          make(map[string]uint64),
		peers:             peers,
	}
}

// set the initial state of the node
func (b *BaselineProtocol) Start(nodeID string) error {
	b.nodeID = nodeID
	b.tick = 0

	for _, p := range b.peers {
		b.lastSeen[p] = 0
	}
	return nil
}

func (b *BaselineProtocol) Tick() []protocol.Envelope {
	b.tick++

	var out []protocol.Envelope

	// periodic heartbeat
	if b.tick%b.heartbeatInterval == 0 {
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

	return out
}

func (b *BaselineProtocol) OnMessage(env protocol.Envelope) []protocol.Envelope {
	// update liveness info
	b.lastSeen[env.From] = b.tick //* At any logical time(tick) I heard from this

	// baseline currently does not respond
	return nil
}

func (b *BaselineProtocol) Stop() error {
	return nil
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

func init() {
	protocol.Register("baseline", func() protocol.ClusterProtocol {
		return NewBaselineProtocol(nil) // this actually returns object of type Baselineprotocol on which we perform operation we store this struct and since this struct impleemnts all the func of the interface it fits perfectly
	})
}
