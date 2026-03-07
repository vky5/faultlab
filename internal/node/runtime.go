package node

import (
	"context"
	"fmt"
	"time"

	"github.com/vky5/faultlab/internal/protocol"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Runtime struct {
	config NodeConfig
	server *grpc.Server
}

func New(cfg NodeConfig) Runtime {
	return Runtime{
		config: cfg,
	}
}

func (r *Runtime) Start() {
	fmt.Printf("starting node %s on port %d\n", r.config.ID, r.config.Port)
	r.server = NewServer(r.config)
	go r.startPingLoop()
	StartGRPCServer(r.config, r.server)
}

func (r *Runtime) Stop() {
	if r.server == nil {
		return
	}

	r.server.GracefulStop()
}

func (r *Runtime) startPingLoop() {
	ticker := time.NewTicker(3 * time.Second)

	for range ticker.C {
		for _, peer := range r.config.Peers {
			go r.pingPeer(peer)
		}
	}
}

func (r *Runtime) pingPeer(peer Peer) {
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
