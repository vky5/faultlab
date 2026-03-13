package manager

import (
	"fmt"
	"time"

	"github.com/vky5/faultlab/internal/cluster"
)

// RegisterNode registers or updates a node in a cluster and returns its peers.
func (m *Manager) RegisterNode(clusterID, nodeID, address string, port int) []cluster.Node {
	m.mu.Lock()
	defer m.mu.Unlock()

	clusterState, ok := m.clusters[clusterID]
	if !ok {
		clusterState = &cluster.Cluster{
			ID:    clusterID,
			Nodes: make(map[string]*cluster.Node),
		}
		m.clusters[clusterID] = clusterState
	}

	node := &cluster.Node{
		ID:       nodeID,
		Address:  address,
		Port:     port,
		LastSeen: time.Now(),
	}
	clusterState.Nodes[nodeID] = node

	peers := make([]cluster.Node, 0, len(clusterState.Nodes))
	for _, n := range clusterState.Nodes {
		if n.ID != nodeID {
			peers = append(peers, *n)
		}
	}
	return peers
}

/*
- remove node func
- remove node gprc
- add cluster
- remove cluster 
*/ 

// RemoveNode removes node from cluster state
func (m *Manager) RemoveNode(clusterID, nodeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	clusterState, ok := m.clusters[clusterID]
	if !ok {
		return fmt.Errorf("cluster id is not valid")
	}

	if _, ok := clusterState.Nodes[nodeID]; !ok {
		return fmt.Errorf("node id is not valid")
	}

	delete(clusterState.Nodes, nodeID)

	return nil
}

func (m *Manager) GetNodes(clusterID string) ([]cluster.Node, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	clusterState, ok := m.clusters[clusterID]
	if !ok {
		return nil, fmt.Errorf("cluster id is not valid")
	}

	nodes := make([]cluster.Node, 0, len(clusterState.Nodes))
	for _, n := range clusterState.Nodes {
		nodes = append(nodes, *n)
	}
	return nodes, nil
}

func (m *Manager) GetNode(clusterID, nodeID string) (*cluster.Node, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()

    c, ok := m.clusters[clusterID]
    if !ok {
        return nil, fmt.Errorf("cluster not found")
    }

    n, ok := c.Nodes[nodeID]
    if !ok {
        return nil, fmt.Errorf("node not found")
    }

    return n, nil
}