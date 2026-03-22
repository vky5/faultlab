package baseline

import (
	"encoding/json"
	"log"
	"strconv"
	"strings"

	"github.com/vky5/faultlab/internal/node/protocol"
)

const (
	baselineColorGreen = "\033[32m"
	colorReset         = "\033[0m"
)

func logBaselinef(format string, args ...any) {
	log.Printf(baselineColorGreen+format+colorReset, args...)
}

type MembershipStatus int

const (
	StatusAlive MembershipStatus = iota
	StatusSuspect
	StatusDead
)

type MembershipEvent struct {
	Node        string
	Status      MembershipStatus
	Incarnation uint64
}

// defines the basic physics (state) of the cluster
type BaselineProtocol struct {
	nodeID string

	tick uint64 // logical time

	heartbeatInterval uint64 // after how many logical ticks heartbeat to send to
	timeoutTicks      uint64

	lastSeen     map[string]uint64
	status       map[string]MembershipStatus
	incarnation  map[string]uint64 // for each peer initial incarnation number is 0 and it is like a self version for their message. they increase it only when they detect that their death notice is circulating. TRhen they send with higher incarnation that notifies other that previous message is old and to overwrite with this one
	suspectSince map[string]uint64

	peers []string

	// Callback for dynamic peer discovery
	peerDiscoveryCb protocol.PeerDiscoveryCallback
}

// creating a baseline
func NewBaselineProtocol(peers []string) *BaselineProtocol {
	logBaselinef("[baseline] NewBaselineProtocol created with %d peers\n", len(peers))
	return &BaselineProtocol{
		heartbeatInterval: 5,
		timeoutTicks:      20,
		lastSeen:          make(map[string]uint64),
		status:            make(map[string]MembershipStatus),
		suspectSince:      make(map[string]uint64),
		incarnation:       make(map[string]uint64),
		peers:             peers,
	}
}

// set the initial state of the node
func (b *BaselineProtocol) Start(nodeID string) error {
	logBaselinef("[baseline] Start called for node %s\n", nodeID)
	b.nodeID = nodeID
	b.tick = 0
	b.status[b.nodeID] = StatusAlive // we are only storing status of Self

	for _, p := range b.peers {
		log.Print(p)
		b.lastSeen[p] = 0
		b.status[p] = StatusAlive
		b.incarnation[p] = 0 // setting incarnation number of all peers to 0
	}
	b.incarnation[b.nodeID] = 0 // setting incarnation number of my node to 0
	logBaselinef("[baseline] Node %s initialized with peers: %v\n", nodeID, b.peers)
	return nil

}

func (b *BaselineProtocol) Tick() []protocol.Envelope {
	b.tick++

	var out []protocol.Envelope

	// periodic heartbeat
	if b.tick%b.heartbeatInterval == 0 {
		logBaselinef("[baseline] Node %s tick %d: sending heartbeats to %d peers\n", b.nodeID, b.tick, len(b.peers))
		for _, peer := range b.peers {
			if b.status[peer] == StatusDead {
				continue
			}

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
		if b.status[peer] == StatusDead {
			continue
		}
		last := b.lastSeen[peer]
		diff := b.tick - last

		switch b.status[peer] {
		case StatusAlive:
			if diff > b.timeoutTicks {
				b.status[peer] = StatusSuspect
				b.suspectSince[peer] = b.tick
				logBaselinef("[baseline] %s SUSPECT at tick %d", peer, b.tick)
				// SUSPECT is not being broadcasted because it is uncertain
			}

		case StatusSuspect:
			if b.tick-b.suspectSince[peer] > b.timeoutTicks { // TODO in SWIM we actually try probing by another peer and only then we mark it dead. this is wayy too agressive fix it later
				b.status[peer] = StatusDead
				logBaselinef("[baseline] %s DEAD at tick %d", peer, b.tick)

				ev := MembershipEvent{ // sending incarnation number with the status dead for a peer
					Node:        peer,
					Status:      StatusDead,
					Incarnation: b.incarnation[peer],
				}

				// DEAD being broadcasted because we are sure that this is the truth
				out = append(out, protocol.Envelope{
					From:     b.nodeID,
					To:       "", // BROADCAST entirety of it in a cluster
					Protocol: "baseline",
					Kind:     protocol.KindProtocol,
					Payload:  encodeEvent(ev),
				})
			}
		}

	}

	return out
}

func (b *BaselineProtocol) OnMessage(env protocol.Envelope) []protocol.Envelope {
	logBaselinef("[baseline] Node %s received message from %q at tick %d\n", b.nodeID, env.From, b.tick)

	senderID := env.From
	var senderHost string
	var senderPort int

	if idx := strings.Index(env.From, "@"); idx != -1 {
		senderID = env.From[:idx]
		addrStr := env.From[idx+1:]
		if colonIdx := strings.LastIndex(addrStr, ":"); colonIdx != -1 {
			senderHost = addrStr[:colonIdx]
			port, _ := strconv.Atoi(addrStr[colonIdx+1:])
			senderPort = port
		}
	}

	// Restore env.From to just the ID so the rest of the logic uses the bare ID
	env.From = senderID

	// update liveness info
	if env.From != b.nodeID {
		logBaselinef("[baseline] message from %q and %d", env.From, b.lastSeen[env.From])
		if _, ok := b.lastSeen[env.From]; !ok {
			logBaselinef("[baseline] discovered new peer %s", env.From)

			b.peers = append(b.peers, env.From)
			b.lastSeen[env.From] = b.tick
			b.status[env.From] = StatusAlive
			b.incarnation[env.From] = 0

			// Notify runtime about peer discovery
			if b.peerDiscoveryCb != nil && senderHost != "" {
				b.peerDiscoveryCb.OnPeerDiscovered(env.From, senderHost, senderPort)
			}
		}

		b.lastSeen[env.From] = b.tick //* At any logical time(tick) I heard from this

		ev, err := decodeEvent(env.Payload)
		if err != nil {
			return nil // for normal heartbeat it will return from here
		}

		localInc := b.incarnation[ev.Node]

		// SELF REVIVAL FIRST
		if ev.Node == b.nodeID && ev.Status == StatusDead && ev.Incarnation >= localInc {
			b.incarnation[b.nodeID]++
			b.status[b.nodeID] = StatusAlive

			logBaselinef("[membership] self revival inc=%d", b.incarnation[b.nodeID])

			return []protocol.Envelope{
				b.makeMembershipEnvelope(b.nodeID, StatusAlive),
			}
		}

		if ev.Incarnation > localInc {
			b.incarnation[ev.Node] = ev.Incarnation
			b.status[ev.Node] = ev.Status
		}
	}

	return nil
}

func (b *BaselineProtocol) Stop() error {
	logBaselinef("[baseline] Stop called for node %s\n", b.nodeID)
	return nil
}

// SetPeers updates the peer list dynamically
func (b *BaselineProtocol) SetPeers(peers []string) {
	logBaselinef("[baseline] SetPeers called: %v\n", peers)
	b.peers = make([]string, len(peers))
	copy(b.peers, peers)

	// Initialize lastSeen for new peers
	for _, p := range peers {
		if _, exists := b.lastSeen[p]; !exists {
			b.lastSeen[p] = 0
			b.status[p] = StatusAlive
			b.incarnation[p] = 0
		}
	}
}

// SetPeerDiscoveryCallback sets the callback for dynamic peer discovery
func (b *BaselineProtocol) SetPeerDiscoveryCallback(cb protocol.PeerDiscoveryCallback) {
	b.peerDiscoveryCb = cb
}

func (b *BaselineProtocol) makeMembershipEnvelope(
	node string,
	status MembershipStatus,
) protocol.Envelope {
	ev := MembershipEvent{
		Node:        node,
		Status:      status,
		Incarnation: b.incarnation[node],
	}

	return protocol.Envelope{
		From:     b.nodeID,
		To:       "", // broadcast
		Protocol: "baseline",
		Kind:     protocol.KindProtocol,
		Payload:  encodeEvent(ev),
	}
}

func (b *BaselineProtocol) State() any {
	return map[string]any{
		"node":        b.nodeID,
		"tick":        b.tick,
		"last_seen":   b.lastSeen,
		"status":      b.status,
		"incarnation": b.incarnation,
		"peers":       b.peers,
	}
}

func (b *BaselineProtocol) hasPeer(id string) bool {
	for _, p := range b.peers {
		if p == id {
			return true
		}
	}
	return false
}

/*
?two nodes in a cluster will not have same time. This is the difference between logical time and real time
?
*/

func init() { // init runs anytime someone imports the module
	logBaselinef("[baseline] init: registering baseline protocol")
	protocol.Register("baseline", func() protocol.ClusterProtocol {
		return NewBaselineProtocol(nil) // this actually returns object of type Baselineprotocol on which we perform operation we store this struct and since this struct impleemnts all the func of the interface it fits perfectly
	})
}

// TODO shift to protobuf or more effective thingy
func encodeEvent(ev MembershipEvent) []byte {
	data, _ := json.Marshal(ev)
	return data
}

func decodeEvent(bts []byte) (MembershipEvent, error) {
	var ev MembershipEvent
	err := json.Unmarshal(bts, &ev)
	return ev, err
}
