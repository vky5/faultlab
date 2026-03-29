package runtime

import (
	"fmt"

	"github.com/vky5/faultlab/internal/protocol"
)

// OnPeerDiscovered handles dynamic peer discovery notifications from the protocol
// layer. It validates and integrates newly discovered peers into the runtime's
// topology if they are not already present in the configuration.
func (r *Runtime) OnPeerDiscovered(peerID, peerHost string, peerPort int) {
	r.logger.Printf("dynamically discovered peer %s at %s:%d", peerID, peerHost, peerPort)

	if r.ctx == nil {
		return
	}
	if peerID == "" || peerPort <= 0 {
		r.logger.Printf("ignoring invalid discovered peer id=%q host=%q port=%d", peerID, peerHost, peerPort)
		return
	}
	if peerHost == "" {
		peerHost = "localhost"
	}

	r.peersMu.RLock()
	base := make([]*protocol.NodeInfo, 0, len(r.config.Peers))
	for _, p := range r.config.Peers {
		base = append(base, &protocol.NodeInfo{Id: p.ID, Address: p.Host, Port: uint32(p.Port)})
	}
	r.peersMu.RUnlock()

	r.applyPeersTopology(base, &protocol.NodeInfo{Id: peerID, Address: peerHost, Port: uint32(peerPort)})
}

// SetFaultParams applies fault injection parameters to this node's runtime,
// including network delay, packet drop rate, partitions, and crash state.
// All parameters are validated before being applied to the fault engine.
func (r *Runtime) SetFaultParams(params *protocol.FaultRequest) error {
	if params == nil {
		return fmt.Errorf("fault params request is nil")
	}

	if r.fault == nil {
		return fmt.Errorf("fault engine is not initialized")
	}

	if params.GetNodeId() != "" && params.GetNodeId() != r.config.ID {
		return fmt.Errorf("fault request node mismatch: got=%s want=%s", params.GetNodeId(), r.config.ID)
	}

	dropRate := params.GetDropRate()
	if dropRate < 0 || dropRate > 1 {
		return fmt.Errorf("drop_rate must be in range [0,1], got=%f", dropRate)
	}

	delayMs := params.GetDelayMs()
	if delayMs < 0 {
		return fmt.Errorf("delay_ms must be >= 0, got=%d", delayMs)
	}

	if params.GetCrashed() {
		r.fault.Crash()
	} else {
		r.fault.Recover()
	}

	r.fault.SetDropRate(dropRate)
	r.fault.SetDelay(int(delayMs))

	// Treat request.partition as the desired partition set.
	desired := make(map[string]struct{}, len(params.GetPartition()))
	for _, id := range params.GetPartition() {
		if id == "" || id == r.config.ID {
			continue
		}
		desired[id] = struct{}{}
	}

	r.peersMu.RLock()
	knownPeers := make([]string, 0, len(r.config.Peers))
	for _, p := range r.config.Peers {
		knownPeers = append(knownPeers, p.ID)
	}
	r.peersMu.RUnlock()

	for _, id := range knownPeers {
		if _, ok := desired[id]; ok {
			r.fault.Partition(id)
		} else {
			r.fault.Heal(id)
		}
	}

	// Also allow partitioning peers that are not currently in runtime config yet.
	for id := range desired {
		r.fault.Partition(id)
	}

	r.logger.Printf("fault params applied: crashed=%v drop_rate=%.3f delay_ms=%d partition=%v", params.GetCrashed(), dropRate, delayMs, params.GetPartition())

	return nil
}
