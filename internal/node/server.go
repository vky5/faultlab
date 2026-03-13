package node

import (
	"context"

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

func (n *NodeRPCServer) Ping(ctx context.Context, _ *protocol.PingRequest) (*protocol.PingResponse, error) {
	return &protocol.PingResponse{
		Message: "Pong",
	}, nil
}

func (n *NodeRPCServer) StopNode(ctx context.Context, _ *protocol.RemoveNodeRequest) (*protocol.RemoveNodeResponse, error) {
	n.nc.Stop()
	return &protocol.RemoveNodeResponse{}, nil
}