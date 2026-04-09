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
		NodeCleanupTimeout: *heartbeatTimeout,
		ProjectRoot:        runtimeCfg.Actor.ProjectRoot,
		DefaultCPHost:      runtimeCfg.Actor.DefaultCPHost,
		DefaultCPPort:      runtimeCfg.Actor.DefaultCPPort,
		NodeBinaryPath:     runtimeCfg.Actor.NodeBinaryPath,
		AppCtx:             appCtx,
	}

	if *configPath != "" {
		cpCfg.Port = runtimeCfg.ControlPlane.Port
		cpCfg.NodeCleanupTimeout = cpTimeout
	}

	// ---- core components ----
	nodeSession := controlplanesession.NewNodeSession(3 * time.Second)
	manager := clustermanager.NewManager(nodeSession)
	nodeClient := controlplanerpc.NewNodeClient(3 * time.Second)

	go manager.Cleanup(cpCfg.NodeCleanupTimeout)

	// ---- service layer ----
	service := controlplanesvc.NewClusterService(manager, nodeClient)

	// ---- actor (command processor) ----
	actor := controlplane.NewActor(manager, service, controlplane.ActorOptions{
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

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("http server stopped: %v", err)
		}
	}()

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
		if err := dispatchRuntimeCommand(raw, actor); err != nil {
			log.Printf("node bootstrap failed for %s: %v", n.ID, err)
		}
	}

	for _, raw := range runtimeCfg.Bootstrap.Commands {
		if err := dispatchRuntimeCommand(raw, actor); err != nil {
			log.Printf("bootstrap command failed (%q): %v", raw, err)
		}
	}

	// ---- CLI command loop ----
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("control plane ready for commands")
	fmt.Println("Commands:")
	fmt.Println("  new-cluster <cluster-id> [--protocol <gossip|raft>] (default: gossip)")
	fmt.Println("  add-node <cluster-id> <node-id> <host> <port>")
	fmt.Println("  remove-node <cluster-id> <node-id>")
	fmt.Println("  list-nodes <cluster-id>")
	fmt.Println("  list-clusters")
	fmt.Println("  set-protocol <cluster-id> <gossip|raft>")
	fmt.Println("  start-node <node-id> <port> [--cluster-id <id>] [--host <host>] [--peers <csv>] [--cp-host <host>] [--cp-port <port>]")
	fmt.Println("  stop-node <node-id>")
	fmt.Println("  list-node-procs")
	fmt.Println("  kv-put <cluster-id> <node-id> <key> <value>")
	fmt.Println("  kv-get <cluster-id> <node-id> <key>")
	fmt.Println("  set-fault <cluster-id> <node-id> <crashed:true|false> <drop-rate:0..1> <delay-ms:int> [partition-csv]")
	fmt.Println("  help")
	go func() {
		for scanner.Scan() {
			if err := dispatchRuntimeCommand(scanner.Text(), actor); err != nil {
				fmt.Println("command error:", err)
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

func dispatchRuntimeCommand(raw string, actor *controlplane.Actor) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}

	parts := strings.Fields(trimmed)
	if len(parts) > 0 && parts[0] == "cp" {
		remainder := strings.TrimSpace(strings.TrimPrefix(trimmed, parts[0]))
		if remainder == "" {
			return fmt.Errorf("usage: cp <controlplane-command...>")
		}
		trimmed = remainder
	}

	cmd, err := controlplane.Parse(trimmed)
	if err != nil {
		return err
	}

	actor.Submit(cmd)

	res, err := cmd.MapWait()
	if err != nil {
		return err
	}

	if res != nil {
		fmt.Printf("result: %+v\n", res)
	} else {
		fmt.Println("ok")
	}

	return nil
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
