package manager

import (
	"fmt"
	"time"
)

// Heartbeat updates liveness for a node.
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
	return nil
}

// Cleanup removes nodes that have not heartbeated within the timeout.
func (m *Manager) Cleanup(timeout time.Duration) {
	for {
		time.Sleep(timeout)

		m.mu.Lock()
		for _, clusterState := range m.clusters {
			for id, node := range clusterState.Nodes {
				if time.Since(node.LastSeen) > timeout {
					delete(clusterState.Nodes, id)
				}
			}
		}
		m.mu.Unlock()
	}
}
