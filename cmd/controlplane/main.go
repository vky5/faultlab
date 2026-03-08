package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"time"

	"google.golang.org/grpc"

	"github.com/vky5/faultlab/internal/cluster"
	"github.com/vky5/faultlab/internal/orchestrator"
	pb "github.com/vky5/faultlab/internal/protocol"
)

func main() {
	port := flag.Int("port", 9000, "gRPC server port")
	heartbeatTimeout := flag.Duration("heartbeat-timeout", 5*time.Second, "node heartbeat timeout")
	flag.Parse()

	manager := cluster.NewManager()

	// cleanup dead nodes
	go manager.Cleanup(*heartbeatTimeout)

	server := orchestrator.NewServer(manager)

	lis, err := net.Listen("tcp", ":"+itoa(*port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()

	pb.RegisterOrchestratorServiceServer(grpcServer, server)

	log.Printf("control plane running on port %d", *port)

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func itoa(v int) string {
	return fmt.Sprintf("%d", v)
}
