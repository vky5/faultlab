package manager

import (
	"fmt"
	"github.com/vky5/faultlab/internal/cluster"
)

// creating an empty cluster
func (m *Manager) CreateCluster(clusterID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.clusters[clusterID]; exists {
		return fmt.Errorf("cluster already exists")
	}

	m.clusters[clusterID] = &cluster.Cluster{
		ID:    clusterID,
		Nodes: make(map[string]*cluster.Node),
	}
	return nil
}

// removing entire cluster
func (m *Manager) RemoveCluster(clusterID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.clusters[clusterID]; !exists {
		return fmt.Errorf("cluster does not exist")
	}

	if nodes, err:= m.GetNodes(clusterID); err == nil {
		for _, node := range nodes {
			m.RemoveNode(clusterID, node.ID)
		}
	}

	delete(m.clusters, clusterID)
	return nil
}

// getting all clusters in the system
func (m *Manager) GetClusters() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	clusterIDs := make([]string, 0, len(m.clusters))
	for id := range m.clusters {
		clusterIDs = append(clusterIDs, id)
	}
	return clusterIDs
}