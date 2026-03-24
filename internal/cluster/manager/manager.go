package manager

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/vky5/faultlab/internal/cluster"
)

type Manager struct {
	mu       sync.RWMutex
	clusters map[string]*cluster.Cluster
	ns       NodeSession
}

func NewManager(ns ...NodeSession) *Manager {
	mgr := &Manager{
		clusters: make(map[string]*cluster.Cluster),
	}

	if len(ns) > 0 {
		mgr.ns = ns[0]
	}

	return mgr
}

type NodeSession interface {
	SetFaultParams(context.Context, string, int, string, cluster.FaultState) error
}

func (m *Manager) SetNodeSession(ns NodeSession) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ns = ns
}

func (m *Manager) SetFaultParams(clusterID, nodeID string, params cluster.FaultState) error {
	m.mu.Lock()

	clusterState, ok := m.clusters[clusterID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("cluster id is not valid")
	}

	node, ok := clusterState.Nodes[nodeID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("node id is not valid")
	}

	node.Fault = params
	host := node.Address
	port := node.Port
	ns := m.ns

	m.mu.Unlock()

	if ns == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return ns.SetFaultParams(ctx, host, port, nodeID, params)
}
