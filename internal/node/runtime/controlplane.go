package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/vky5/faultlab/internal/protocol"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func (r *Runtime) controlPlaneSyncLoop() {
	if err := r.syncWithControlPlane(); err != nil {
		fmt.Printf("control-plane sync failed: %v\n", err) // register the node to the control plane
	}

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if err := r.syncWithControlPlane(); err != nil { // get the peer list (this will overwrite the peer list)
			fmt.Printf("control-plane sync failed: %v\n", err)
		}
	}
}

func (r *Runtime) syncWithControlPlane() error {
	if err := r.registerNodeWithControlPlane(); err != nil {
		return err
	}
	return r.getPeersFromControlplane()
}



// Registering the node to the server
func (r *Runtime) registerNodeWithControlPlane() error {
	addr := fmt.Sprintf("%s:%d", r.config.ControlPlaneHost, r.config.ControlPlanePort)

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
	r.peersMu.Unlock()

	return nil
}
