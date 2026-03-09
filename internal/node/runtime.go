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

// controls wht happens in nodes during starting
func (r *Runtime) Start() {
	fmt.Printf("starting node %s on port %d\n", r.config.ID, r.config.Port)

	err := r.registerNodeWithControlPlane()
	if err != nil {
		fmt.Println("node registration failed:", err)
		return
	}

	fmt.Println("node successfully registered")

	r.server = NewServer(r.config)

	go r.startPingLoop()
	StartGRPCServer(r.config, r.server)
}

// to stop the RPC server
func (r *Runtime) Stop() {
	if r.server == nil {
		return
	}

	r.server.GracefulStop()
}

// loop over peer list to call pingPeer
func (r *Runtime) startPingLoop() {
	ticker := time.NewTicker(3 * time.Second)

	for range ticker.C {
		for _, peer := range r.config.Peers {
			go r.pingPeer(peer)
		}
	}
}

// Ping all peer list nodes
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

// Registering the node to the server
func (r *Runtime) registerNodeWithControlPlane() error {
	addr := fmt.Sprintf("%s:%d", r.config.ControlPlaneHost, r.config.ControlPlanePort)


	conn, err := grpc.NewClient( // connecting to orchestrator
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)

	if err!=nil {
		return err
	}

	defer conn.Close()

	client := protocol.NewOrchestratorServiceClient(conn)	 // getting client object of the NewOrchestratorService

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := client.RegisterNode(ctx, &protocol.RegisterNodeRequest{ // calling the registered func of the client 
		ClusterId: r.config.ClusterID,
		NodeId:    r.config.ID,
		Address:   r.config.Host,
		Port:      int32(r.config.Port),
	})


	if err != nil {
		return err
	}

	if resp.Status != protocol.RegisterStatus_SUCCESS {
		return fmt.Errorf("registration rejected: %s", resp.Message)
	}

	return nil
}
