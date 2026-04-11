package metrics

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// SessionManager keeps in-memory metrics sessions keyed by cluster id.
type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

// Session stores tracked keys and snapshots for one cluster run.
type Session struct {
	ClusterID string
	StartedAt time.Time
	StoppedAt *time.Time

	trackedKeys map[string]struct{}
	snapshots   map[string][]Snapshot
}

// SessionSnapshot is a read-only view of a session state.
type SessionSnapshot struct {
	ClusterID   string
	StartedAt   time.Time
	StoppedAt   *time.Time
	TrackedKeys []string
}

// NewSessionManager creates an empty in-memory session manager.
func NewSessionManager() *SessionManager {
	return &SessionManager{sessions: make(map[string]*Session)}
}

// Start begins a metrics session for a cluster.
// Returns an error if an active session already exists.
func (m *SessionManager) Start(clusterID string, now time.Time) error {
	clusterID = strings.TrimSpace(clusterID)
	if clusterID == "" {
		return fmt.Errorf("cluster id is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, ok := m.sessions[clusterID]; ok && existing.StoppedAt == nil {
		return fmt.Errorf("metrics session already active for cluster %q", clusterID)
	}

	m.sessions[clusterID] = &Session{
		ClusterID:   clusterID,
		StartedAt:   now,
		trackedKeys: make(map[string]struct{}),
		snapshots:   make(map[string][]Snapshot),
	}

	return nil
}

// Stop ends an active metrics session.
func (m *SessionManager) Stop(clusterID string, now time.Time) error {
	clusterID = strings.TrimSpace(clusterID)
	if clusterID == "" {
		return fmt.Errorf("cluster id is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sessions[clusterID]
	if !ok {
		return fmt.Errorf("no metrics session for cluster %q", clusterID)
	}
	if s.StoppedAt != nil {
		return fmt.Errorf("metrics session already stopped for cluster %q", clusterID)
	}

	stopped := now
	s.StoppedAt = &stopped
	return nil
}

// IsActive reports whether a cluster has an active metrics session.
func (m *SessionManager) IsActive(clusterID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[strings.TrimSpace(clusterID)]
	return ok && s.StoppedAt == nil
}

// TrackKey registers a key for metrics collection under an active session.
func (m *SessionManager) TrackKey(clusterID, key string) error {
	clusterID = strings.TrimSpace(clusterID)
	key = strings.TrimSpace(key)
	if clusterID == "" {
		return fmt.Errorf("cluster id is required")
	}
	if key == "" {
		return fmt.Errorf("key is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sessions[clusterID]
	if !ok {
		return fmt.Errorf("no metrics session for cluster %q", clusterID)
	}
	if s.StoppedAt != nil {
		return fmt.Errorf("metrics session is stopped for cluster %q", clusterID)
	}

	s.trackedKeys[key] = struct{}{}
	if _, exists := s.snapshots[key]; !exists {
		s.snapshots[key] = []Snapshot{}
	}
	return nil
}

// RecordSnapshot appends one key snapshot into the active session.
func (m *SessionManager) RecordSnapshot(clusterID, key string, observedAt time.Time, nodes map[string]NodeState) error {
	clusterID = strings.TrimSpace(clusterID)
	key = strings.TrimSpace(key)
	if clusterID == "" {
		return fmt.Errorf("cluster id is required")
	}
	if key == "" {
		return fmt.Errorf("key is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sessions[clusterID]
	if !ok {
		return fmt.Errorf("no metrics session for cluster %q", clusterID)
	}
	if s.StoppedAt != nil {
		return fmt.Errorf("metrics session is stopped for cluster %q", clusterID)
	}

	s.trackedKeys[key] = struct{}{}
	if _, exists := s.snapshots[key]; !exists {
		s.snapshots[key] = []Snapshot{}
	}

	t := observedAt.Sub(s.StartedAt)
	if t < 0 {
		t = 0
	}

	copyNodes := make(map[string]NodeState, len(nodes))
	for nodeID, state := range nodes {
		copyNodes[nodeID] = state
	}

	s.snapshots[key] = append(s.snapshots[key], Snapshot{Time: t, Nodes: copyNodes})
	return nil
}

// GetSession returns a read-only snapshot of one session.
func (m *SessionManager) GetSession(clusterID string) (SessionSnapshot, error) {
	clusterID = strings.TrimSpace(clusterID)
	if clusterID == "" {
		return SessionSnapshot{}, fmt.Errorf("cluster id is required")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.sessions[clusterID]
	if !ok {
		return SessionSnapshot{}, fmt.Errorf("no metrics session for cluster %q", clusterID)
	}

	keys := make([]string, 0, len(s.trackedKeys))
	for key := range s.trackedKeys {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var stopped *time.Time
	if s.StoppedAt != nil {
		copyStopped := *s.StoppedAt
		stopped = &copyStopped
	}

	return SessionSnapshot{
		ClusterID:   s.ClusterID,
		StartedAt:   s.StartedAt,
		StoppedAt:   stopped,
		TrackedKeys: keys,
	}, nil
}

// ComputeKeyResult computes metrics for one tracked key in a session.
func (m *SessionManager) ComputeKeyResult(clusterID, key string) (Result, error) {
	clusterID = strings.TrimSpace(clusterID)
	key = strings.TrimSpace(key)
	if clusterID == "" {
		return Result{}, fmt.Errorf("cluster id is required")
	}
	if key == "" {
		return Result{}, fmt.Errorf("key is required")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.sessions[clusterID]
	if !ok {
		return Result{}, fmt.Errorf("no metrics session for cluster %q", clusterID)
	}

	series, ok := s.snapshots[key]
	if !ok {
		return Result{}, fmt.Errorf("key %q is not tracked for cluster %q", key, clusterID)
	}

	copied := make([]Snapshot, len(series))
	for i := range series {
		copied[i] = Snapshot{Time: series[i].Time, Nodes: make(map[string]NodeState, len(series[i].Nodes))}
		for nodeID, state := range series[i].Nodes {
			copied[i].Nodes[nodeID] = state
		}
	}

	return Compute(copied), nil
}

// ComputeAllResults computes metrics for all tracked keys in a session.
func (m *SessionManager) ComputeAllResults(clusterID string) (map[string]Result, error) {
	clusterID = strings.TrimSpace(clusterID)
	if clusterID == "" {
		return nil, fmt.Errorf("cluster id is required")
	}

	m.mu.RLock()
	s, ok := m.sessions[clusterID]
	if !ok {
		m.mu.RUnlock()
		return nil, fmt.Errorf("no metrics session for cluster %q", clusterID)
	}

	keys := make([]string, 0, len(s.snapshots))
	for key := range s.snapshots {
		keys = append(keys, key)
	}
	seriesByKey := make(map[string][]Snapshot, len(s.snapshots))
	for _, key := range keys {
		series := s.snapshots[key]
		copied := make([]Snapshot, len(series))
		for i := range series {
			copied[i] = Snapshot{Time: series[i].Time, Nodes: make(map[string]NodeState, len(series[i].Nodes))}
			for nodeID, state := range series[i].Nodes {
				copied[i].Nodes[nodeID] = state
			}
		}
		seriesByKey[key] = copied
	}
	m.mu.RUnlock()

	out := make(map[string]Result, len(seriesByKey))
	for key, series := range seriesByKey {
		out[key] = Compute(series)
	}

	return out, nil
}
