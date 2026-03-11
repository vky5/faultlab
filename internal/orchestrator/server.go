package orchestrator

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/vky5/faultlab/internal/cluster"
	clustermanager "github.com/vky5/faultlab/internal/cluster/manager"
	pb "github.com/vky5/faultlab/internal/protocol"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	grpcpeer "google.golang.org/grpc/peer"
)

type Server struct {
	pb.UnimplementedOrchestratorServiceServer
	manager *clustermanager.Manager

	nodeClient *NodeClient
}

func NewServer(m *clustermanager.Manager, nc *NodeClient) *Server {
	return &Server{manager: m, nodeClient: nc}
}

// registering a node to control plane
func (s *Server) RegisterNode(
	ctx context.Context,
	req *pb.RegisterNodeRequest,
) (*pb.RegisterNodeResponse, error) {
	log.Printf("register request: cluster=%s node=%s addr=%s:%d",
		req.ClusterId, req.NodeId, req.Address, req.Port)

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
	pCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	client := pb.NewNodeServiceClient(conn)
	_, err = client.Ping(pCtx, &pb.PingRequest{From: "orchestrator"})
	if err != nil {
		return &pb.RegisterNodeResponse{
			Status:  pb.RegisterStatus_FAILED,
			Message: fmt.Sprintf("node verification failed: %v", err),
		}, nil
	}

	peers := s.manager.RegisterNode(
		req.ClusterId,
		req.NodeId,
		req.Address,
		int(req.Port),
	)
	log.Printf("new node added: cluster=%s node=%s addr=%s:%d peers_now=%d",
		req.ClusterId, req.NodeId, req.Address, req.Port, len(peers)+1)

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
	peerIDs := make([]string, 0, len(nodes))
	for _, n := range nodes {
		peers = append(peers, &pb.NodeInfo{
			Id:      n.ID,
			Address: n.Address,
			Port:    uint32(n.Port),
		})
		peerIDs = append(peerIDs, n.ID)
	}

	requester := "unknown"
	if p, ok := grpcpeer.FromContext(ctx); ok && p.Addr != nil {
		requester = p.Addr.String()
	}
	log.Printf("get-peers served: requester=%s cluster=%s count=%d peers=[%s]",
		requester, req.ClusterId, len(peers), strings.Join(peerIDs, ", "))

	return &pb.PeersResponse{
		Peers: peers,
	}, nil
}

func (s *Server) RemoveNode(
	ctx context.Context,
	req *pb.RemoveNodeRequest,
) (*pb.RemoveNodeResponse, error) {
	log.Printf("remove-node request: cluster=%s node=%s", req.ClusterId, req.NodeId)

	nodes, err := s.manager.GetNodes(req.ClusterId)
	if err != nil {
		return nil, err
	}

	var target *cluster.Node

	for _, n := range nodes {
		if n.ID == req.NodeId {
			target = &n
			break
		}
	}

	if target == nil {
		return nil, fmt.Errorf("node not found")
	}

	// stop node process
	if err := s.nodeClient.StopNode(ctx, target.Address, target.Port); err != nil {
		return nil, err
	}
	log.Printf("stop-node sent: node=%s addr=%s:%d", req.NodeId, target.Address, target.Port)

	// remove from cluster state
	if err := s.manager.RemoveNode(req.ClusterId, req.NodeId); err != nil {
		return nil, err
	}
	log.Printf("node removed from cluster state: cluster=%s node=%s", req.ClusterId, req.NodeId)

	return &pb.RemoveNodeResponse{}, nil
}
