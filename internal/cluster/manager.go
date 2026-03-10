package cluster

import (
	"fmt"
	"sync"
	"time"
)

type Manager struct {
	mu       sync.RWMutex
	clusters map[string]*Cluster
}

func NewManager() *Manager {
	return &Manager{
		clusters: make(map[string]*Cluster),
	}
}

// register a new node to control plane in a cluster
func (m *Manager) RegisterNode(clusterID, nodeID, address string, port int) []Node {
	m.mu.Lock() // read and write both lock
	defer m.mu.Unlock()

	cluster, err := m.clusters[clusterID]
	if !err {
		cluster = &Cluster{
			ID:    clusterID,
			Nodes: make(map[string]*Node),
		}

		m.clusters[clusterID] = cluster
	}

	node := &Node{
		ID:       nodeID,
		Address:  address,
		Port:     port,
		LastSeen: time.Now(),
	}

	cluster.Nodes[nodeID] = node

	peers := make([]Node, 0)
	for _, n := range cluster.Nodes {
		if n.ID != nodeID {
			peers = append(peers, *n)
		}
	}

	return peers

}

// node will periodically send their status to heartbeat
func (m *Manager) Heartbeat(clusterID, nodeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cluster, ok := m.clusters[clusterID]
	if !ok {
		return fmt.Errorf("cluster id is not valid")
	}
	node, ok := cluster.Nodes[nodeID]
	if !ok {
		return fmt.Errorf("node id is not valid")
	}

	node.LastSeen = time.Now()

	return nil
}

func (m *Manager) GetNodes(clusterID string) ([]Node, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cluster, ok := m.clusters[clusterID]
	if !ok {
		return nil, fmt.Errorf("cluster id is not valid")
	}

	nodes := make([]Node, 0, len(cluster.Nodes))
	for _, n := range cluster.Nodes {
		nodes = append(nodes, *n)
	}

	return nodes, nil
}

// for removing the nodes if they don't respond in certain time
func (m *Manager) Cleanup(timeout time.Duration) {
	for {
		time.Sleep(timeout)
		m.mu.Lock()

		for _, cluster := range m.clusters {
			for id, node := range cluster.Nodes {
				if time.Since(node.LastSeen) > timeout {
					delete(cluster.Nodes, id)
				}
			}
		}

		m.mu.Unlock()
	}
}


