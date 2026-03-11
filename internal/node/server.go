package node

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/vky5/faultlab/internal/protocol"
	"google.golang.org/grpc"
)

type NodeController interface {
	Stop()
}

type NodeRPCServer struct {
	nc NodeController
	protocol.UnimplementedNodeServiceServer
}

func NewServer(nc NodeController) *grpc.Server {
	server := grpc.NewServer()
	protocol.RegisterNodeServiceServer(server, &NodeRPCServer{
		nc: nc,
	})
	return server

}

func StartGRPCServer(cfg NodeConfig, server *grpc.Server) {

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		log.Fatalf("TCP init failed: %v", err)
	}

	fmt.Printf("Node %s listening on %d\n", cfg.ID, cfg.Port)

	if err := server.Serve(lis); err != nil {
		log.Fatalf("failed to serve : %v", err)
	}
}

func (n *NodeRPCServer) Ping(ctx context.Context, _ *protocol.PingRequest) (*protocol.PingResponse, error) {
	return &protocol.PingResponse{
		Message: "Pong",
	}, nil
}


func (n *NodeRPCServer) StopNode(ctx context.Context, _ *protocol.RemoveNodeRequest) (*protocol.RemoveNodeResponse, error) {
	n.nc.Stop()
	return &protocol.RemoveNodeResponse{}, nil
}	