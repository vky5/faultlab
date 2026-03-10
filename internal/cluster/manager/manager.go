package manager

import (
	"sync"

	"github.com/vky5/faultlab/internal/cluster"
)

type Manager struct {
	mu       sync.RWMutex
	clusters map[string]*cluster.Cluster
}

func NewManager() *Manager {
	return &Manager{
		clusters: make(map[string]*cluster.Cluster),
	}
}
