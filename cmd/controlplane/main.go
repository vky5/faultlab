package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"google.golang.org/grpc"

	clustermanager "github.com/vky5/faultlab/internal/cluster/manager"
	"github.com/vky5/faultlab/internal/controlplane"
	"github.com/vky5/faultlab/internal/controlplane/rest"
	controlplanerpc "github.com/vky5/faultlab/internal/controlplane/rpc"
	controlplanesvc "github.com/vky5/faultlab/internal/controlplane/service"
	controlplanesession "github.com/vky5/faultlab/internal/controlplane/session"
	"github.com/vky5/faultlab/internal/engine"
	pb "github.com/vky5/faultlab/internal/protocol"
)

func main() {
	// ---- flags ----
	configPath := flag.String("config", "", "path to runtime yaml config")
	port := flag.Int("port", 9000, "control plane gRPC port")
	httpPort := flag.Int("http-port", 8080, "control plane HTTP port")
	commandPort := flag.Int("command-port", 9091, "control plane command ingestion HTTP port")
	commandAuthToken := flag.String("command-auth-token", "", "optional bearer token for command ingestion API")
	heartbeatTimeout := flag.Duration(
		"heartbeat-timeout",
		5*time.Second,
		"node heartbeat timeout",
	)
	flag.Parse()

	appCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	runtimeCfg := engine.DefaultRuntimeConfig()
	if *configPath != "" {
		loaded, err := engine.LoadRuntimeConfig(*configPath)
		if err != nil {
			log.Fatalf("failed to load runtime config: %v", err)
		}
		runtimeCfg = loaded
	}

	cpTimeout, err := runtimeCfg.ControlPlaneTimeout()
	if err != nil {
		log.Fatalf("invalid runtime config: %v", err)
	}

	cpCfg := &controlplane.ControlPlane{
		Port:               *port,
		CommandPort:        *commandPort,
		CommandAuthToken:   *commandAuthToken,
		NodeCleanupTimeout: *heartbeatTimeout,
		ProjectRoot:        runtimeCfg.Actor.ProjectRoot,
		DefaultCPHost:      runtimeCfg.Actor.DefaultCPHost,
		DefaultCPPort:      runtimeCfg.Actor.DefaultCPPort,
		NodeBinaryPath:     runtimeCfg.Actor.NodeBinaryPath,
		AppCtx:             appCtx,
	}

	if *configPath != "" {
		cpCfg.Port = runtimeCfg.ControlPlane.Port
		cpCfg.CommandPort = runtimeCfg.ControlPlane.CommandPort
		cpCfg.CommandAuthToken = runtimeCfg.ControlPlane.CommandAuthToken
		cpCfg.NodeCleanupTimeout = cpTimeout
	}

	if *commandAuthToken != "" {
		cpCfg.CommandAuthToken = *commandAuthToken
	}

	// ---- core components ----
	nodeSession := controlplanesession.NewNodeSession(3 * time.Second)
	manager := clustermanager.NewManager(nodeSession)
	nodeClient := controlplanerpc.NewNodeClient(3 * time.Second)

	go manager.Cleanup(cpCfg.NodeCleanupTimeout)

	// ---- service layer ----
	service := controlplanesvc.NewClusterService(manager, nodeClient)

	// ---- actor (command processor) ----
	actor := controlplane.NewActor(service, controlplane.ActorOptions{
		ProjectRoot:    cpCfg.ProjectRoot,
		DefaultCPHost:  cpCfg.DefaultCPHost,
		DefaultCPPort:  cpCfg.DefaultCPPort,
		NodeBinaryPath: cpCfg.NodeBinaryPath,
	})
	go actor.Run()

	// ---- gRPC server ----
	grpcServer := grpc.NewServer()
	orchestratorServer := controlplanerpc.NewServer(service)

	pb.RegisterOrchestratorServiceServer(grpcServer, orchestratorServer)

	addr := fmt.Sprintf(":%d", cpCfg.Port)

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

	// listening for HTTP requests
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("http server stopped: %v", err)
		}
	}()

	commandServer := controlplane.StartCommandListener(actor, controlplane.CommandListenerConfig{
		Port:      cpCfg.CommandPort,
		AuthToken: cpCfg.CommandAuthToken,
	})

	// ---- bootstrap nodes and execute commands ----
	for _, n := range runtimeCfg.Nodes {
		if n.ID == "" || n.Port <= 0 {
			log.Printf("skipping invalid node config: id=%q port=%d", n.ID, n.Port)
			continue
		}

		raw := fmt.Sprintf("start-node %s %d --cluster-id %s --host %s --peers %s --cp-host %s --cp-port %d",
			n.ID,
			n.Port,
			normalizeDefault(n.ClusterID, "default"),
			normalizeDefault(n.Host, "localhost"),
			n.PeersCSV,
			normalizeDefault(n.CPHost, cpCfg.DefaultCPHost),
			normalizeDefaultInt(n.CPPort, cpCfg.DefaultCPPort),
		)
		if err := controlplane.DispatchRuntimeCommand(raw, actor); err != nil {
			log.Printf("node bootstrap failed for %s: %v", n.ID, err)
		}
	}

	// execute bootstrap commands by DispatchRuntimeCommand
	for _, raw := range runtimeCfg.Bootstrap.Commands {
		if err := controlplane.DispatchRuntimeCommand(raw, actor); err != nil {
			log.Printf("bootstrap command failed (%q): %v", raw, err)
		}
	}

	fmt.Println("control plane ready")
	fmt.Println("use cpcli for interactive help and command execution")

	// Terminating the program
	<-appCtx.Done()
	log.Printf("shutting down control plane")

	actor.Stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = httpServer.Shutdown(shutdownCtx)
	if commandServer != nil {
		_ = commandServer.Shutdown(shutdownCtx)
	}
	grpcServer.GracefulStop()
}

func normalizeDefault(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func normalizeDefaultInt(value int, fallback int) int {
	if value == 0 {
		return fallback
	}
	return value
}
