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

// Cleanup removes nodes when heartbeat timeout is exceeded.
func (m *Manager) Cleanup(timeout time.Duration) {
	for {
		time.Sleep(timeout)

		m.mu.Lock()
		for _, clusterState := range m.clusters {
			for id, node := range clusterState.Nodes {
				if time.Since(node.LastSeen) > timeout {
					// Only remove if not artificially crashed
					// If node.Fault.Crashed is true, node is still running but fault-injected
					// If node.Fault.Crashed is false, node is actually dead
					if !node.Fault.Crashed {
						// Node is actually dead (Ctrl+C or crash), remove it
						delete(clusterState.Nodes, id)
					} else {
						// Node is artificially crashed but still running, keep it
						node.Status = "crashed"
						clusterState.Nodes[id] = node
					}
				}
			}
		}
		m.mu.Unlock()
	}
}
