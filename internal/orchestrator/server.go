package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/vky5/faultlab/internal/cluster"
	pb "github.com/vky5/faultlab/internal/protocol"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)



type Server struct {
	pb.UnimplementedOrchestratorServiceServer
	manager *cluster.Manager
}


func NewServer(m *cluster.Manager) *Server{
	return &Server{manager: m}
}



// registering a node to control plane 
func (s *Server) RegisterNode(
	ctx context.Context,
	req *pb.RegisterNodeRequest,
) (*pb.RegisterNodeResponse, error) {
	// Reachability Check: Try to dial the node to see if it's actually there
	addr := fmt.Sprintf("%s:%d", req.Address, req.Port)
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return &pb.RegisterNodeResponse{
			Status:  pb.RegisterStatus_FAILED,
			Message: fmt.Sprintf("failed to reach node for verification: %v", err),
		}, nil
	}
	defer conn.Close()

	// Add timeout for verification
	pCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	client := pb.NewNodeServiceClient(conn)
	_, err = client.Ping(pCtx, &pb.PingRequest{From: "orchestrator"})
	if err != nil {
		return &pb.RegisterNodeResponse{
			Status:  pb.RegisterStatus_FAILED,
			Message: fmt.Sprintf("node verification failed: %v", err),
		}, nil
	}

	s.manager.RegisterNode(
		req.ClusterId,
		req.NodeId,
		req.Address,
		int(req.Port),
	)

	return &pb.RegisterNodeResponse{
		Status:  pb.RegisterStatus_SUCCESS,
		Message: "node registered",
	}, nil
}


// getting the peers of a cluster (including node making request)
func (s *Server) GetPeers(ctx context.Context, req *pb.PeersRequest) (*pb.PeersResponse, error) {
	nodes, err := s.manager.GetNodes(req.ClusterId)
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

	return &pb.PeersResponse{
		Peers: peers,
	}, nil
}