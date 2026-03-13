package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"time"

	"google.golang.org/grpc"

	clustermanager "github.com/vky5/faultlab/internal/cluster/manager"
	controlplanerpc "github.com/vky5/faultlab/internal/controlplane/rpc"
	controlplanesvc "github.com/vky5/faultlab/internal/controlplane/service"
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
	nodeClient := controlplanerpc.NewNodeClient(3 * time.Second)

	go manager.Cleanup(*heartbeatTimeout)

	// ---- service layer ----
	service := controlplanesvc.NewClusterService(manager, nodeClient)

	// ---- gRPC server ----
	grpcServer := grpc.NewServer()
	orchestratorServer := controlplanerpc.NewServer(service)

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
	// TODO: integrate with service layer when actor is ready
	// scanner := bufio.NewScanner(os.Stdin)
	// fmt.Println("control plane ready for commands")
	// for scanner.Scan() {
	// 	input := scanner.Text()
	// 	cmd, err := controlplane.Parse(input)
	// 	if err != nil {
	// 		fmt.Println("command error:", err)
	// 		continue
	// 	}
	// 	actor.Submit(cmd)
	// }

	log.Println("control plane running (CLI disabled)")
	select {} // block forever
}
