package runtime

import (
	"context"
	"fmt"
	"strings"
	"time"
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
	return r.cp.Establish(parentCtx)
}

// getPeersFromControlplane fetches latest peer topology.
func (r *Runtime) getPeersFromControlplane(parentCtx context.Context) error {
	peers, err := r.cp.FetchPeers(parentCtx)
	if err != nil {
		return err
	}

	r.peersMu.Lock()
	r.config.SetPeers(peers)

	peersStr := make([]string, 0, len(r.config.Peers))
	for _, p := range r.config.Peers {
		peersStr = append(peersStr, fmt.Sprintf("%s@%s:%d", p.ID, p.Host, p.Port))
	}
	r.peersMu.Unlock()

	fmt.Printf("[node:%s] peers updated: count=%d peers=[%s]\n",
		r.config.ID, len(peersStr), strings.Join(peersStr, ", "))

	return nil
}

// sendHeartbeatToControlPlane reports liveness.
func (r *Runtime) sendHeartbeatToControlPlane(parentCtx context.Context) error {
	if err := r.cp.Heartbeat(parentCtx); err != nil {
		return err
	}

	fmt.Printf("[node:%s] heartbeat sent\n", r.config.ID)
	return nil
}
