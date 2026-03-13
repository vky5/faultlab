// This code snippet is defining a set of functions related to managing a peer-to-peer network
// communication system. Here's a breakdown of what each part of the code is doing:
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




// loop over peer list to call pingPeer
func (r *Runtime) startPingLoop() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		for _, peer := range r.snapshotPeers() {
			go r.pingPeer(peer)
		}
	}
}

func (r *Runtime) snapshotPeers() []node.Peer {
	r.peersMu.RLock()
	defer r.peersMu.RUnlock()

	peers := make([]node.Peer, len(r.config.Peers))
	copy(peers, r.config.Peers)
	return peers
}

// Ping all peer list nodes
func (r *Runtime) pingPeer(peer node.Peer) {
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	resp, err := client.Ping(ctx, &protocol.PingRequest{
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
