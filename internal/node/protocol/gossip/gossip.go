package gossip

import (
	"fmt"
	"log"
	"time"

	"github.com/vky5/faultlab/internal/node/protocol"
)

const (
	gossipColorCyan  = "\033[36m"
	gossipColorReset = "\033[0m"
)

func (g *GossipProtocol) logGossipf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if err := appendGossipFileLog(msg); err != nil {
		log.Printf("[gossip] file log failed: %v", err)
	}
	log.Printf(gossipColorCyan+"[gossip] %s"+gossipColorReset, msg)
	if g.logger != nil {
		if l, ok := g.logger.(interface{ Printf(string, ...any) }); ok {
			l.Printf(msg)
		}
	}
}

func (g *GossipProtocol) emitTimelineEvent(eventType, key, value string, version int64, origin, source string, ts int64) {
	// Format: TL_EVENT:<TYPE>|<KEY>|<VALUE>|<VERSION>|<ORIGIN>|<SOURCE>|<ROUND>|<TIMESTAMP>
	// We use wall-clock time for <ROUND> now.
	msg := fmt.Sprintf("TL_EVENT:%s|%s|%s|%d|%s|%s|%d|%d", eventType, key, value, version, origin, source, time.Now().UnixMilli(), ts)
	if err := appendGossipFileLog(msg); err != nil {
		log.Printf("[gossip] file log failed: %v", err)
	}
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

	forceSyncPeers map[string]bool // peers to actively sync with (e.g., after partition heal)

	logger any
}

func NewGossipProtocol() *GossipProtocol {
	g := &GossipProtocol{
		nodeID:         "",
		peers:          []string{},
		store:          make(map[string]Value),
		gossipInterval: 3,
		forceSyncPeers: make(map[string]bool),
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

// for setting up peers
func (g *GossipProtocol) SetPeers(peers []string) {
	filtered := make([]string, 0, len(peers))
	newPeers := make(map[string]bool)

	for _, p := range peers {
		if p != "" && p != g.nodeID {
			filtered = append(filtered, p)
			newPeers[p] = true
		}
	}

	// Detect new peers (from partition heal) and mark them for active sync
	oldPeersMap := make(map[string]bool)
	for _, p := range g.peers {
		oldPeersMap[p] = true
	}

	for peer := range newPeers {
		if !oldPeersMap[peer] {
			// This is a new peer, mark for immediate sync
			g.forceSyncPeers[peer] = true
			g.logGossipf("SetPeers detected new peer %s (likely from partition heal), marking for active sync", peer)
		}
	}

	g.peers = filtered
	g.logGossipf("SetPeers called for node %s: %v (forceSyncPeers: %v)", g.nodeID, g.peers, g.forceSyncPeers)
}

// SetPeerDiscoveryCallback sets the callback for dynamic peer discovery.
func (g *GossipProtocol) SetPeerDiscoveryCallback(cb protocol.PeerDiscoveryCallback) {
	g.peerDiscoveryCb = cb
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

func (g *GossipProtocol) SetLogger(l any) {
	g.logger = l
}

func init() {
	protocol.Register("gossip", func() protocol.ClusterProtocol {
		return NewGossipProtocol()
	})
}
