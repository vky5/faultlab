package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"

	clustermanager "github.com/vky5/faultlab/internal/cluster/manager"
	"github.com/vky5/faultlab/internal/controlplane"
	"github.com/vky5/faultlab/internal/controlplane/rest"
	controlplanerpc "github.com/vky5/faultlab/internal/controlplane/rpc"
	controlplanesvc "github.com/vky5/faultlab/internal/controlplane/service"
	controlplanesession "github.com/vky5/faultlab/internal/controlplane/session"
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

	appCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// ---- core components ----
	nodeSession := controlplanesession.NewNodeSession(3 * time.Second)
	manager := clustermanager.NewManager(nodeSession)
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
			log.Printf("grpc server stopped: %v", err)
		}
	}()

	// ---- HTTP server ----
	restServer := rest.NewServer(actor)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/clusters", restServer.HandleClusters)
	mux.HandleFunc("/api/clusters/", restServer.HandleNodes) // handle nested paths

	httpAddr := fmt.Sprintf(":%d", *httpPort)
	httpServer := &http.Server{Addr: httpAddr, Handler: mux}
	log.Printf("control plane HTTP listening on %s", httpAddr)

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("http server stopped: %v", err)
		}
	}()

	// ---- CLI command loop ----
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("control plane ready for commands")
	fmt.Println("Commands:")
	fmt.Println("  new-cluster <cluster-id> [--protocol <gossip|raft>] (default: gossip)")
	fmt.Println("  add-node <cluster-id> <node-id> <host> <port>")
	fmt.Println("  remove-node <cluster-id> <node-id>")
	fmt.Println("  list-nodes <cluster-id>")
	fmt.Println("  list-clusters")
	fmt.Println("  kv-put <cluster-id> <node-id> <key> <value>")
	fmt.Println("  kv-get <cluster-id> <node-id> <key>")
	fmt.Println("  set-fault <cluster-id> <node-id> <crashed:true|false> <drop-rate:0..1> <delay-ms:int> [partition-csv]")
	fmt.Println("  help")
	go func() {
		for scanner.Scan() {
			input := scanner.Text()
			cmd, err := controlplane.Parse(input)
			if err != nil {
				fmt.Println("command error:", err)
				continue
			}

			actor.Submit(cmd)

			res, err := cmd.MapWait()
			if err != nil {
				fmt.Println("command failed:", err)
				continue
			}

			if res != nil {
				fmt.Printf("result: %+v\n", res)
			} else {
				fmt.Println("ok")
			}
		}
	}()

	<-appCtx.Done()
	log.Printf("shutting down control plane")

	actor.Stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = httpServer.Shutdown(shutdownCtx)
	grpcServer.GracefulStop()
}
