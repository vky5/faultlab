package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"google.golang.org/grpc"

	clustermanager "github.com/vky5/faultlab/internal/cluster/manager"
	controlplane "github.com/vky5/faultlab/internal/controlplane"
	controlplanerpc "github.com/vky5/faultlab/internal/controlplane/rpc"
	pb "github.com/vky5/faultlab/internal/protocol"
)

func main() {
	// ---- flags ----
	port := flag.Int("port", 9000, "control plane gRPC port")
	heartbeatTimeout := flag.Duration(
		"heartbeat-timeout",
		5*time.Second,
		"node heartbeat timeout",
	)
	flag.Parse()

	// ---- core components ----
	manager := clustermanager.NewManager()

	go manager.Cleanup(*heartbeatTimeout)

	// ---- actor ----
	actor := controlplane.NewActor(manager)

	go actor.Run()

	// ---- gRPC server ----
	grpcServer := grpc.NewServer()

	orchestratorServer := controlplanerpc.NewServer(manager)

	pb.RegisterOrchestratorServiceServer(grpcServer, orchestratorServer)

	addr := fmt.Sprintf(":%d", *port)

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("control plane listen failed: %v", err)
	}

	log.Printf("control plane listening on %s", addr)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("grpc server stopped: %v", err)
		}
	}()

	// ---- CLI command loop ----
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("control plane ready for commands")

	for scanner.Scan() {

		input := scanner.Text()

		cmd, err := controlplane.Parse(input)
		if err != nil {
			fmt.Println("command error:", err)
			continue
		}

		actor.Submit(cmd)
	}

	if err := scanner.Err(); err != nil {
		log.Println("stdin error:", err)
	}
}
