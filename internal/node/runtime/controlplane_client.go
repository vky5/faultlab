package runtime

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/vky5/faultlab/internal/protocol"
)

// controlPlaneSyncLoop manages periodic sync with controlplane.
// Lifecycle is controlled by runtime context.
func (r *Runtime) controlPlaneSyncLoop(ctx context.Context) {
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
		r.logger.Printf("heartbeat failed: %v", err)
		return
	}

}

// registerNodeWithControlPlane performs initial registration.
func (r *Runtime) registerNodeWithControlPlane(parentCtx context.Context) (string, error) {
	assignedProtocol, err := r.cp.Establish(parentCtx)
	if err != nil {
		return "", err
	}

	r.logger.Printf("controlplane assigned protocol: %s", assignedProtocol)
	return assignedProtocol, nil
}

// fetchPeersFromControlplane only fetches topology from controlplane.
func (r *Runtime) fetchPeersFromControlplane(parentCtx context.Context) ([]*protocol.NodeInfo, error) {
	return r.cp.FetchPeers(parentCtx)
}

// applyPeersTopology reconciles runtime/session/protocol state.
// If discovered is non-nil, it is merged into the input topology first.
func (r *Runtime) applyPeersTopology(peers []*protocol.NodeInfo, discovered *protocol.NodeInfo) {
	if discovered != nil {
		merged := make([]*protocol.NodeInfo, 0, len(peers)+1)
		found := false
		for _, p := range peers {
			if p.Id == discovered.Id {
				found = true
				host := p.Address
				port := p.Port
				if discovered.Address != "" {
					host = discovered.Address
				}
				if discovered.Port > 0 {
					port = discovered.Port
				}
				merged = append(merged, &protocol.NodeInfo{Id: discovered.Id, Address: host, Port: port})
				continue
			}
			merged = append(merged, p)
		}

		if !found {
			merged = append(merged, discovered)
		}

		peers = merged
	}

	r.peersMu.Lock()
	r.config.SetPeers(peers)

	peersStr := make([]string, 0, len(r.config.Peers))
	for _, p := range r.config.Peers {
		peersStr = append(peersStr, fmt.Sprintf("%s@%s:%d", p.ID, p.Host, p.Port))
	}
	r.peersMu.Unlock()

	r.logger.Printf("peers updated: count=%d peers=[%s]",
		len(peersStr), strings.Join(peersStr, ", "))

	// Runtime passes peer topology to session
	// Session owns connection lifecycle: decides what to add/remove
	sessionPeers := make([]PeerInfo, 0, len(r.config.Peers))
	for _, p := range r.config.Peers {
		sessionPeers = append(sessionPeers, PeerInfo{
			ID:   p.ID,
			Host: p.Host,
			Port: p.Port,
		})
	}
	r.ns.OnPeersUpdated(sessionPeers)

	// Notify protocol about peer changes
	peerIDs := make([]string, 0, len(r.config.Peers))
	for _, p := range r.config.Peers {
		peerIDs = append(peerIDs, p.ID)
	}
	if peerAwareProtocol, ok := r.proto.(interface{ SetPeers([]string) }); ok {
		peerAwareProtocol.SetPeers(peerIDs)
	}

}

// getPeersFromControlplane fetches latest peer topology and applies it across runtime layers.
func (r *Runtime) getPeersFromControlplane(parentCtx context.Context) error {
	peers, err := r.fetchPeersFromControlplane(parentCtx)
	if err != nil {
		return err
	}

	r.applyPeersTopology(peers, nil)

	return nil
}

// sendHeartbeatToControlPlane reports liveness.
func (r *Runtime) sendHeartbeatToControlPlane(parentCtx context.Context) error {
	tick := r.BeforeTick()
	if !tick.Allow {
		r.logger.Printf("heartbeat skipped: node fault gate closed (%s)", tick.Reason)
		return nil
	}

	if err := r.cp.Heartbeat(parentCtx); err != nil {
		return err
	}

	// r.logger.Printf("heartbeat sent") // Too noisy to pipe over RPC
	return nil
}

// reportCapabilitiesToControlPlane reports the currently active protocol and action contract.
func (r *Runtime) reportCapabilitiesToControlPlane(parentCtx context.Context, protocolKey string, epoch uint64) error {
	if protocolKey == "" {
		return fmt.Errorf("protocol key is required for capability reporting")
	}

	if r.proto == nil {
		return fmt.Errorf("cannot report capabilities: protocol is not initialized")
	}

	capabilities := CheckCapabilities(r.proto)
	if err := r.cp.ReportNodeCapabilities(parentCtx, protocolKey, epoch, capabilities); err != nil {
		return err
	}

	r.logger.Printf("capabilities reported: protocol=%s epoch=%d actions=%+v", protocolKey, epoch, capabilities.Actions)
	return nil
}
