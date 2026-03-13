package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/vky5/faultlab/internal/node"
	"github.com/vky5/faultlab/internal/protocol"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// startPingLoop runs periodic peer pinging.
// Lifecycle is controlled by runtime context.
func (r *Runtime) startPingLoop(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done(): // triggered on r.cancel()
			return
		case <-ticker.C:
			r.runPingCycle(ctx)
		}
	}
}

// runPingCycle performs one ping sweep across peers.
func (r *Runtime) runPingCycle(ctx context.Context) {
	for _, peer := range r.snapshotPeers() {
		// sequential for now (no goroutine fan-out yet)
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
func (r *Runtime) pingPeer(parentCtx context.Context, peer node.Peer) {
	addr := fmt.Sprintf("%s:%d", peer.Host, peer.Port)

	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		fmt.Printf("dial failed %s (%s): %v\n", peer.ID, addr, err)
		return
	}
	defer conn.Close()

	client := protocol.NewNodeServiceClient(conn)

	// operation-scoped timeout derived from runtime ctx
	// basically either this timesout or other cancels out
	opCtx, cancel := context.WithTimeout(parentCtx, time.Second)
	defer cancel()

	resp, err := client.Ping(opCtx, &protocol.PingRequest{
		From: r.config.ID,
	})
	if err != nil {
		fmt.Printf("ping %s (%s) failed: %v\n", peer.ID, addr, err)
		return
	}

	fmt.Printf("%s → ping → %s (%s)\n",
		r.config.ID,
		peer.ID,
		resp.Message,
	)
}