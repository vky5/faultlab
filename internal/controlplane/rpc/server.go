package rpc

import (
	"context"
	"fmt"
	"strings"

	controlplanesvc "github.com/vky5/faultlab/internal/controlplane/service"
	pb "github.com/vky5/faultlab/internal/protocol"
)

type Server struct {
	pb.UnimplementedOrchestratorServiceServer
	svc *controlplanesvc.Service
}

// the sole purpose of the rpc server should be to recieve the request, validate it and call the service
// manager <- Service <- Server (-> = calls )

func NewServer(svc *controlplanesvc.Service) *Server {
	return &Server{svc: svc}
}

// registering a node to control plane
func (s *Server) RegisterNode(
	ctx context.Context,
	req *pb.RegisterNodeRequest,
) (*pb.RegisterNodeResponse, error) {

	err := s.svc.RegisterNode(
		ctx,
		req.ClusterId,
		req.NodeId,
		req.Address,
		int(req.Port),
	)

	if err != nil {
		return &pb.RegisterNodeResponse{
			Status:  pb.RegisterStatus_FAILED,
			Message: err.Error(),
		}, nil
	}

	protocolName, err := s.svc.GetClusterProtocol(req.ClusterId)
	if err != nil {
		return &pb.RegisterNodeResponse{
			Status:  pb.RegisterStatus_FAILED,
			Message: err.Error(),
		}, nil
	}

	return &pb.RegisterNodeResponse{
		Status:   pb.RegisterStatus_SUCCESS,
		Message:  "node registered",
		Protocol: mapSupportedProtocol(protocolName), // the protocl is assigned at the time of creating a cluster
	}, nil
}

func mapSupportedProtocol(name string) pb.SupportedProtocol {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "gossip":
		return pb.SupportedProtocol_SUPPORTED_PROTOCOL_GOSSIP
	case "raft":
		return pb.SupportedProtocol_SUPPORTED_PROTOCOL_RAFT
	default:
		return pb.SupportedProtocol_SUPPORTED_PROTOCOL_UNSPECIFIED
	}
}

// getting the peers of a cluster (including node making request)
func (s *Server) GetPeers(ctx context.Context, req *pb.PeersRequest) (*pb.PeersResponse, error) {

	nodes, err := s.svc.GetPeers(req.ClusterId)
	if err != nil {
		return nil, err
	}

	peers := make([]*pb.NodeInfo, 0, len(nodes))
	for _, n := range nodes {
		peers = append(peers, &pb.NodeInfo{
			Id:      n.ID,
			Address: n.Address,
			Port:    uint32(n.Port),
		})
	}

	return &pb.PeersResponse{Peers: peers}, nil
}

// Heartbeat updates LastSeen for a node in a cluster.
func (s *Server) Heartbeat(
	ctx context.Context,
	req *pb.HeartbeatRequest,
) (*pb.HeartbeatResponse, error) {

	err := s.svc.Heartbeat(req.ClusterId, req.Id)
	if err != nil {
		return &pb.HeartbeatResponse{Ok: false}, err
	}

	return &pb.HeartbeatResponse{Ok: true}, nil
}

// ReportLog receives log entries from a node and prints them to the Control Plane console.
func (s *Server) ReportLog(
	ctx context.Context,
	req *pb.LogRequest,
) (*pb.LogResponse, error) {

	// Provide visual trace in the control plane directly
	fmt.Printf("[ControlPlane Intercept] Node: %s | [%s] %s\n",
		req.NodeId, req.Level, req.Message)

	// Stream to any active SSE listeners mapping UI visuals
	s.svc.BroadcastLog(req)

	return &pb.LogResponse{Ok: true}, nil
}
