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

func (n *NodeRPCServer) Handshake(ctx context.Context, req *protocol.HandshakeRequest) (*protocol.HandshakeResponse, error) {
	if req == nil {
		return &protocol.HandshakeResponse{Success: false, Message: "handshake request is nil"}, nil
	}

	if req.GetNodeId() == "" {
		return &protocol.HandshakeResponse{Success: false, Message: "node_id is required"}, nil
	}
	if req.GetAddr() == "" {
		return &protocol.HandshakeResponse{Success: false, Message: "addr is required"}, nil
	}
	if req.GetPort() <= 0 {
		return &protocol.HandshakeResponse{Success: false, Message: "port must be > 0"}, nil
	}

	return &protocol.HandshakeResponse{Success: true, Message: "handshake successful"}, nil
}

func (n *NodeRPCServer) StopNode(ctx context.Context, _ *protocol.RemoveNodeRequest) (*protocol.RemoveNodeResponse, error) {
	n.nc.Stop()
	return &protocol.RemoveNodeResponse{}, nil
}


// func (n *NodeRPCServer) Send(ctx context.Context)|=