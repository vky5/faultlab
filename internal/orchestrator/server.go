package orchestrator

	"context"
	"fmt"
	"net"

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



// 
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
	pCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond) // short timeout
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

func (s *Server) GetPeers(ctx context.Context, req *pb.PeersRequest) (*pb.PeersResponse, error) {
	// Note: Currently manager.RegisterNode returns peers, but we don't have a direct GetNodes for a cluster yet.
	// Since we are single-cluster for now as per docs, we can just return all nodes from the first cluster or similar.
	// However, manager.go shows it handles multiple clusters.
	// Ideally GetPeers should take a cluster_id.
	// For now, let's keep it simple as per existing proto.
	return &pb.PeersResponse{}, nil
}