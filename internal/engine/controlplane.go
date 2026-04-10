package engine

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	clustermanager "github.com/vky5/faultlab/internal/cluster/manager"
	"github.com/vky5/faultlab/internal/controlplane"
	controlplanerpc "github.com/vky5/faultlab/internal/controlplane/rpc"
	controlplanesvc "github.com/vky5/faultlab/internal/controlplane/service"
	controlplanesession "github.com/vky5/faultlab/internal/controlplane/session"
	pb "github.com/vky5/faultlab/internal/protocol"
	"google.golang.org/grpc"
)

func (e *Engine) NewControlplane() {
	if e == nil || e.ControlPlane == nil {
		log.Println("engine or controlplane config is nil; skipping controlplane startup")
		return
	}

	appCtx := e.ControlPlane.AppCtx
	if appCtx == nil {
		appCtx = context.Background()
	}

	nodeSession := controlplanesession.NewNodeSession(3 * time.Second)
	manager := clustermanager.NewManager(nodeSession)
	nodeClient := controlplanerpc.NewNodeClient(3 * time.Second)

	go manager.Cleanup(e.ControlPlane.NodeCleanupTimeout) // cleanup node if the last heartbeat heard was greater than NodeTImeOut

	// ---- service layer ----
	service := controlplanesvc.NewClusterService(manager, nodeClient)
	actor := controlplane.NewActor(service, controlplane.ActorOptions{
		ProjectRoot:    e.ControlPlane.ProjectRoot,
		DefaultCPHost:  e.ControlPlane.DefaultCPHost,
		DefaultCPPort:  e.ControlPlane.DefaultCPPort,
		NodeBinaryPath: e.ControlPlane.NodeBinaryPath,
	})
	go actor.Run()

	// --- gRPC server ---
	grpcServer := grpc.NewServer()
	orchestratorServer := controlplanerpc.NewServer(service)

	pb.RegisterOrchestratorServiceServer(grpcServer, orchestratorServer)

	addr := fmt.Sprintf(":%d", e.ControlPlane.Port)
	fmt.Printf("Starting control plane gRPC server on %s\n", addr)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	log.Printf("controlplane listening on %s", addr)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Printf("controlplane gRPC server stopped: %v", err)
		}
	}()

	if e.ControlPlane.CommandCh != nil {
		go forwardCommandsToActor(e.ControlPlane.CommandCh, actor)
	}

	<-appCtx.Done()
	log.Println("shutting down control plane gRPC server...")
	actor.Stop()
	grpcServer.GracefulStop()
}

func forwardCommandsToActor(commandCh <-chan string, actor *controlplane.Actor) {
	for raw := range commandCh {
		cmd, err := controlplane.Parse(raw)
		if err != nil {
			log.Printf("command parse error (%q): %v", raw, err)
			continue
		}

		actor.Submit(cmd)

		res, err := cmd.MapWait()
		if err != nil {
			log.Printf("command failed (%q): %v", raw, err)
			continue
		}

		if res != nil {
			log.Printf("command result (%q): %+v", raw, res)
		}
	}
}
