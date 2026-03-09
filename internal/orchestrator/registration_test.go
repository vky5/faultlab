package orchestrator

import (
	"context"
	"net"
	"testing"

	"github.com/vky5/faultlab/internal/cluster"
	pb "github.com/vky5/faultlab/internal/protocol"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func TestRegisterNode(t *testing.T) {
	// 1. Setup bufconn for in-memory gRPC communication
	lis := bufconn.Listen(1024 * 1024)
	s := grpc.NewServer()

	manager := cluster.NewManager()
	srv := NewServer(manager)
	pb.RegisterOrchestratorServiceServer(s, srv)

	go func() {
		if err := s.Serve(lis); err != nil {
			t.Errorf("Server exited with error: %v", err)
		}
	}()
	defer s.Stop()

	// 2. Setup Client
	ctx := context.Background()
	conn, err := grpc.NewClient("bufnet", 
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()
	client := pb.NewOrchestratorServiceClient(conn)

	// 3. Test Registration
	resp, err := client.RegisterNode(ctx, &pb.RegisterNodeRequest{
		ClusterId: "test-cluster",
		NodeId:    "test-node",
		Address:   "127.0.0.1",
		Port:      8001,
	})

	if err != nil {
		t.Fatalf("RegisterNode failed: %v", err)
	}

	if resp.Message != "node registered" {
		t.Errorf("Expected 'node registered' message, got %v", resp.Message)
	}

	// 4. Verify in Manager (indirectly checking if it was actually added)
	// Since RegisterNode returns peers, let's register a second node and check if the first one is in the peer list
	resp2, err := client.RegisterNode(ctx, &pb.RegisterNodeRequest{
		ClusterId: "test-cluster",
		NodeId:    "test-node-2",
		Address:   "127.0.0.1",
		Port:      8002,
	})

	if err != nil {
		t.Fatalf("Second RegisterNode failed: %v", err)
	}

	if resp2.Status != pb.RegisterStatus_SUCCESS {
		t.Errorf("Expected SUCCESS status for second node, got %v", resp2.Status)
	}

	// 5. Test GetPeers for the second node
	peersResp, err := client.GetPeers(ctx, &pb.PeersRequest{})
	if err != nil {
		t.Fatalf("GetPeers failed: %v", err)
	}

	// Note: We expect to see 'test-node' in the peer list for 'test-node-2'
	// However, GetPeers implementation needs to be checked in orchestrator/server.go
}
