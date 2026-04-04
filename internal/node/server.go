// handlers for the gRPC that node exposes

package node

import (
	"context"

	"github.com/vky5/faultlab/internal/node/exec"
	"github.com/vky5/faultlab/internal/protocol"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type noderuntime interface { // this is what runtime implements  check runtime.go
	Stop()
	HandleEnvelope(env *protocol.EnvelopeRequest)
	ExecuteAction(ctx context.Context, req *protocol.ActionRequest) (*protocol.ActionResponse, error)
	SetFaultParams(params *protocol.FaultRequest) error
	ProtocolSwap(ctx context.Context, proto string) error
	exec.FaultDecider
}

type NodeRPCServer struct {
	nc noderuntime
	protocol.UnimplementedNodeServiceServer
}

func NewServer(nc noderuntime) *grpc.Server {
	server := grpc.NewServer()

	protocol.RegisterNodeServiceServer(server, &NodeRPCServer{
		nc: nc,
	})

	return server
}

func (n *NodeRPCServer) Ping(ctx context.Context, _ *protocol.PingRequest) (*protocol.PingResponse, error) {
	if d := n.nc.BeforeTick(); !d.Allow {
		return nil, status.Error(codes.Unavailable, "node is crashed")
	}

	return &protocol.PingResponse{
		Message: "Pong",
	}, nil
}

func (n *NodeRPCServer) Handshake(ctx context.Context, req *protocol.HandshakeRequest) (*protocol.HandshakeResponse, error) {
	if d := n.nc.BeforeTick(); !d.Allow {
		return nil, status.Error(codes.Unavailable, "node is crashed")
	}

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

// recieving the messages from the other remote nodes
func (n *NodeRPCServer) SendEnvelope(
	ctx context.Context,
	req *protocol.EnvelopeRequest,
) (*protocol.EnvelopeAck, error) {
	if d := n.nc.BeforeTick(); !d.Allow {
		return nil, status.Error(codes.Unavailable, "node is crashed")
	}

	if req == nil {
		return &protocol.EnvelopeAck{
			Success: false,
			Message: "nil envelope",
		}, nil
	}

	n.nc.HandleEnvelope(req) // func in runtime

	return &protocol.EnvelopeAck{
		Success: true,
		Message: "delivered",
	}, nil
}

func (n *NodeRPCServer) SetFaultParams(
	ctx context.Context,
	req *protocol.FaultRequest,
) (*protocol.FaultResponse, error) {
	if req == nil {
		return &protocol.FaultResponse{
			Success: false,
			Message: "nil fault request",
		}, nil
	}

	if err := n.nc.SetFaultParams(req); err != nil {
		return &protocol.FaultResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &protocol.FaultResponse{
		Success: true,
		Message: "fault parameters applied",
	}, nil
}

// ExecuteAction takes the action request from the controlplane and send it to runtime for execution
func (n *NodeRPCServer) ExecuteAction(
	ctx context.Context,
	req *protocol.ActionRequest,
) (*protocol.ActionResponse, error) {

	if d := n.nc.BeforeTick(); !d.Allow {
		return nil, status.Error(codes.Unavailable, "node is crashed")
	}

	if req == nil {
		return &protocol.ActionResponse{
			Success: false,
			Message: "nil action request",
		}, nil
	}

	resp, err := n.nc.ExecuteAction(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// handling protocol change
func (n *NodeRPCServer) ChangeProtocol(ctx context.Context, req *protocol.ChangeProtocolRequest) (*protocol.ChangeProtocolResponse, error) {
	if d := n.nc.BeforeTick(); !d.Allow {
		return nil, status.Error(codes.Unavailable, "node is crashed")
	}

	if req == nil {
		return &protocol.ChangeProtocolResponse{
			Done:    false,
			Message: "nil change protocol request",
		}, nil
	}

	assigned := req.GetAssignedProtocol()
	if assigned == nil {
		return &protocol.ChangeProtocolResponse{
			Done:    false,
			Message: "assigned_protocol is required",
		}, nil
	}

	key := assigned.GetKey()
	if key == "" {
		return &protocol.ChangeProtocolResponse{
			Done:    false,
			Message: "assigned_protocol.key is required",
		}, nil
	}

	if err := n.nc.ProtocolSwap(ctx, key); err != nil {
		return &protocol.ChangeProtocolResponse{
			Done:           false,
			ActiveProtocol: assigned,
			Message:        err.Error(),
		}, nil
	}

	return &protocol.ChangeProtocolResponse{
		Done:           true,
		ActiveProtocol: assigned,
		Message:        "protocol change acknowledged",
	}, nil
}
