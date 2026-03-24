package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"google.golang.org/grpc"

	clustermanager "github.com/vky5/faultlab/internal/cluster/manager"
	"github.com/vky5/faultlab/internal/controlplane"
	"github.com/vky5/faultlab/internal/controlplane/rest"
	controlplanerpc "github.com/vky5/faultlab/internal/controlplane/rpc"
	controlplanesvc "github.com/vky5/faultlab/internal/controlplane/service"
	pb "github.com/vky5/faultlab/internal/protocol"
)

func main() {
	// ---- flags ----
	port := flag.Int("port", 9000, "control plane gRPC port")
	httpPort := flag.Int("http-port", 8080, "control plane HTTP port")
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

	// ---- actor (command processor) ----
	actor := controlplane.NewActor(manager, service)
	go actor.Run()

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

	// ---- HTTP server ----
	restServer := rest.NewServer(actor)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/clusters", restServer.HandleClusters)
	mux.HandleFunc("/api/clusters/", restServer.HandleNodes) // handle nested paths

	httpAddr := fmt.Sprintf(":%d", *httpPort)
	log.Printf("control plane HTTP listening on %s", httpAddr)

	go func() {
		if err := http.ListenAndServe(httpAddr, mux); err != nil {
			log.Fatalf("http server stopped: %v", err)
		}
	}()

	// ---- CLI command loop ----
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("control plane ready for commands")
	fmt.Println("Commands: new-cluster <id> <protocol>, add-node <cluster> <node> <host> <port>, list-nodes <cluster>, remove-node <cluster> <node>")
	for scanner.Scan() {
		input := scanner.Text()
		cmd, err := controlplane.Parse(input)
		if err != nil {
			fmt.Println("command error:", err)
			continue
		}
		actor.Submit(cmd)
	}

	// If stdin is closed (e.g., when run in background), block forever so the servers keep running
	select {}
}
