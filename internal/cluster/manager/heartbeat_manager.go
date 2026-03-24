package manager

import (
	"fmt"
	"time"
)

// Heartbeat updates lastseen for a node.
func (m *Manager) Heartbeat(clusterID, nodeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	clusterState, ok := m.clusters[clusterID]
	if !ok {
		return fmt.Errorf("cluster id is not valid")
	}

	node, ok := clusterState.Nodes[nodeID]
	if !ok {
		return fmt.Errorf("node id is not valid")
	}

	node.LastSeen = time.Now()
	node.Status = "active"
	return nil
}

// Cleanup marks nodes dead when heartbeat timeout is exceeded.
func (m *Manager) Cleanup(timeout time.Duration) {
	for {
		time.Sleep(timeout)

		m.mu.Lock()
		for _, clusterState := range m.clusters {
			for id, node := range clusterState.Nodes {
				if time.Since(node.LastSeen) > timeout {
					node.Status = "dead"
					clusterState.Nodes[id] = node
				}
			}
		}
		m.mu.Unlock()
	}
}
