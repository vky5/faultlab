package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/vky5/faultlab/internal/node"
)

// startPingLoop runs periodic peer pinging.
// Lifecycle is controlled by runtime context.
func (r *Runtime) startPingLoop(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.runPingCycle(ctx)
		}
	}
}

// runPingCycle performs one ping sweep across peers.
func (r *Runtime) runPingCycle(ctx context.Context) {
	for _, peer := range r.snapshotPeers() {
		r.pingPeer(ctx, peer)
	}
}

// snapshotPeers returns a safe copy of current peer list.
func (r *Runtime) snapshotPeers() []node.Peer {
	r.peersMu.RLock()
	defer r.peersMu.RUnlock()

	peers := make([]node.Peer, len(r.config.Peers))
	copy(peers, r.config.Peers)
	return peers
}

// pingPeer performs a single ping operation with bounded timeout.
func (r *Runtime) pingPeer(ctx context.Context, peer node.Peer) {
	if err := r.ns.Ping(ctx, peer.ID, peer.Host, peer.Port); err != nil {
		fmt.Printf("ping %s (%s:%d) failed: %v\n", peer.ID, peer.Host, peer.Port, err)
	}
}
