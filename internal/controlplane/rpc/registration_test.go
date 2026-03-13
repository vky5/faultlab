package rpc

import (
	"context"
	"net"
	"testing"

	clustermanager "github.com/vky5/faultlab/internal/cluster/manager"
	pb "github.com/vky5/faultlab/internal/protocol"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func TestRegisterNode(t *testing.T) {
	// 1. Setup bufconn for in-memory gRPC communication
	lis := bufconn.Listen(1024 * 1024)
	s := grpc.NewServer()

	manager := clustermanager.NewManager()
	srv := NewServer(manager, nil)
	pb.RegisterOrchestratorServiceServer(s, srv)

	go func() {
		if err := s.Serve(lis); err != nil {
			t.Errorf("Server exited with error: %v", err)
		}
	}()
	defer s.Stop()

	// 2. Setup Client
	ctx := context.Background()
	conn, err := grpc.NewClient("passthrough:///bufnet",
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

	// 3. Setup a mock node to handle verification Ping
	nodeLis, err := net.Listen("tcp", "127.0.0.1:8001")
	if err != nil {
		t.Fatalf("Failed to listen for mock node: %v", err)
	}
	nodeSrv := grpc.NewServer()
	pb.RegisterNodeServiceServer(nodeSrv, &mockNodeServer{})
	go nodeSrv.Serve(nodeLis)
	defer nodeSrv.Stop()

	// 4. Test Registration
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

	// Setup second mock node
	nodeLis2, err := net.Listen("tcp", "127.0.0.1:8002")
	if err != nil {
		t.Fatalf("Failed to listen for mock node 2: %v", err)
	}
	nodeSrv2 := grpc.NewServer()
	pb.RegisterNodeServiceServer(nodeSrv2, &mockNodeServer{})
	go nodeSrv2.Serve(nodeLis2)
	defer nodeSrv2.Stop()

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
	peersResp, err := client.GetPeers(ctx, &pb.PeersRequest{ClusterId: "test-cluster"})
	if err != nil {
		t.Fatalf("GetPeers failed: %v", err)
	}

	foundPeer := false
	for _, p := range peersResp.Peers {
		if p.Id == "test-node" {
			foundPeer = true
			break
		}
	}
	if !foundPeer {
		t.Errorf("Expected to find 'test-node' in peer list")
	}
}

type mockNodeServer struct {
	pb.UnimplementedNodeServiceServer
}

func (m *mockNodeServer) Ping(ctx context.Context, req *pb.PingRequest) (*pb.PingResponse, error) {
	return &pb.PingResponse{Message: "Pong"}, nil
}
