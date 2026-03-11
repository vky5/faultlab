package runtime

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/vky5/faultlab/internal/protocol"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func (r *Runtime) controlPlaneSyncLoop() {
	fmt.Printf("[node:%s] control-plane sync loop started (cluster=%s, cp=%s:%d)\n",
		r.config.ID, r.config.ClusterID, r.config.ControlPlaneHost, r.config.ControlPlanePort)
	if err := r.registerNodeWithControlPlane(); err != nil {
		fmt.Printf("[node:%s] startup registration failed: %v\n", r.config.ID, err)
		return
	}

	if err := r.syncWithControlPlane(); err != nil {
		fmt.Printf("[node:%s] initial sync failed: %v\n", r.config.ID, err)
	}

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if err := r.syncWithControlPlane(); err != nil {
			fmt.Printf("[node:%s] periodic sync failed: %v\n", r.config.ID, err)
		}
	}
}

func (r *Runtime) syncWithControlPlane() error {
	if err := r.sendHeartbeatToControlPlane(); err != nil {
		return err
	}
	return r.getPeersFromControlplane()
}

// Registering the node to the server
func (r *Runtime) registerNodeWithControlPlane() error {
	addr := fmt.Sprintf("%s:%d", r.config.ControlPlaneHost, r.config.ControlPlanePort)
	fmt.Printf("[node:%s] registering with control-plane=%s cluster=%s host=%s port=%d\n",
		r.config.ID, addr, r.config.ClusterID, r.config.Host, r.config.Port)

	conn, err := grpc.NewClient( // connecting to orchestrator
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)

	if err != nil {
		return err
	}

	defer conn.Close()

	client := protocol.NewOrchestratorServiceClient(conn) // getting client object of the NewOrchestratorService

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := client.RegisterNode(ctx, &protocol.RegisterNodeRequest{ // calling the registered func of the client
		ClusterId: r.config.ClusterID,
		NodeId:    r.config.ID,
		Address:   r.config.Host,
		Port:      int32(r.config.Port),
	})

	if err != nil {
		return err
	}

	if resp.Status != protocol.RegisterStatus_SUCCESS {
		return fmt.Errorf("registration rejected: %s", resp.Message)
	}

	fmt.Printf("[node:%s] registration successful\n", r.config.ID)
	return nil
}

// getting the peer list and upgrading it
func (r *Runtime) getPeersFromControlplane() error {
	addr := fmt.Sprintf("%s:%d", r.config.ControlPlaneHost, r.config.ControlPlanePort)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}

	defer conn.Close()

	client := protocol.NewOrchestratorServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := client.GetPeers(ctx, &protocol.PeersRequest{ClusterId: r.config.ClusterID})

	if err != nil {
		return err
	}

	r.peersMu.Lock()
	r.config.SetPeers(resp.Peers)
	peers := make([]string, 0, len(r.config.Peers))
	for _, p := range r.config.Peers {
		peers = append(peers, fmt.Sprintf("%s@%s:%d", p.ID, p.Host, p.Port))
	}
	r.peersMu.Unlock()

	fmt.Printf("[node:%s] peers updated: count=%d peers=[%s]\n",
		r.config.ID, len(peers), strings.Join(peers, ", "))

	return nil
}

// sendHeartbeatToControlPlane updates LastSeen for this node in control-plane state.
func (r *Runtime) sendHeartbeatToControlPlane() error {
	addr := fmt.Sprintf("%s:%d", r.config.ControlPlaneHost, r.config.ControlPlanePort)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	client := protocol.NewOrchestratorServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := client.Heartbeat(ctx, &protocol.HeartbeatRequest{
		Id:        r.config.ID,
		ClusterId: r.config.ClusterID,
	})
	if err != nil {
		return err
	}
	if !resp.Ok {
		return fmt.Errorf("heartbeat rejected")
	}

	fmt.Printf("[node:%s] heartbeat sent\n", r.config.ID)
	return nil
}
