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

// controlPlaneSyncLoop manages periodic sync with controlplane.
// Lifecycle is controlled by runtime context.
func (r *Runtime) controlPlaneSyncLoop(ctx context.Context) {

	// first registration (blocking, before loop)
	if err := r.registerNodeWithControlPlane(ctx); err != nil {
		fmt.Printf("[node:%s] registration failed: %v\n", r.config.ID, err)
	}

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.runControlplaneSyncCycle(ctx)
		}
	}
}

// runControlplaneSyncCycle performs one full sync cycle.
func (r *Runtime) runControlplaneSyncCycle(ctx context.Context) {
	if err := r.sendHeartbeatToControlPlane(ctx); err != nil {
		fmt.Printf("[node:%s] heartbeat failed: %v\n", r.config.ID, err)
		return
	}

	if err := r.getPeersFromControlplane(ctx); err != nil {
		fmt.Printf("[node:%s] peer sync failed: %v\n", r.config.ID, err)
	}
}

// registerNodeWithControlPlane performs initial registration.
func (r *Runtime) registerNodeWithControlPlane(parentCtx context.Context) error {

	addr := fmt.Sprintf("%s:%d", r.config.ControlPlaneHost, r.config.ControlPlanePort)

	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := protocol.NewOrchestratorServiceClient(conn)

	opCtx, cancel := context.WithTimeout(parentCtx, 3*time.Second)
	defer cancel()

	resp, err := client.RegisterNode(opCtx, &protocol.RegisterNodeRequest{
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

// getPeersFromControlplane fetches latest peer topology.
func (r *Runtime) getPeersFromControlplane(parentCtx context.Context) error {

	addr := fmt.Sprintf("%s:%d", r.config.ControlPlaneHost, r.config.ControlPlanePort)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	client := protocol.NewOrchestratorServiceClient(conn)

	opCtx, cancel := context.WithTimeout(parentCtx, 3*time.Second)
	defer cancel()

	resp, err := client.GetPeers(opCtx, &protocol.PeersRequest{
		ClusterId: r.config.ClusterID,
	})
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

// sendHeartbeatToControlPlane reports liveness.
func (r *Runtime) sendHeartbeatToControlPlane(parentCtx context.Context) error {

	addr := fmt.Sprintf("%s:%d", r.config.ControlPlaneHost, r.config.ControlPlanePort)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	client := protocol.NewOrchestratorServiceClient(conn)

	opCtx, cancel := context.WithTimeout(parentCtx, 3*time.Second)
	defer cancel()

	resp, err := client.Heartbeat(opCtx, &protocol.HeartbeatRequest{
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