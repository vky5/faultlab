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
	sessions map[string][]*Session
}

// SessionStats stores session-specific throughput data.
type SessionStats struct {
	TotalRPCs   uint64
	TotalWrites uint64
	RecentKeys  []string
}

// Session stores tracked keys and snapshots for one cluster run.
type Session struct {
	ClusterID string
	StartedAt time.Time
	StoppedAt *time.Time

	trackedKeys map[string]struct{}
	snapshots   map[string][]Snapshot
	timeline    map[string][]TimelineEvent
	Stats       SessionStats
}

// SessionSnapshot is a read-only view of a session state.
type SessionSnapshot struct {
	ClusterID   string
	StartedAt   time.Time
	StoppedAt   *time.Time
	TrackedKeys []string
	Stats       SessionStats
}

// NewSessionManager creates an empty in-memory session manager.
func NewSessionManager() *SessionManager {
	return &SessionManager{sessions: make(map[string][]*Session)}
}

// Start begins a metrics session for a cluster.
func (m *SessionManager) Start(clusterID string, now time.Time) error {
	clusterID = strings.TrimSpace(clusterID)
	if clusterID == "" {
		return fmt.Errorf("cluster id is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	history := m.sessions[clusterID]
	if len(history) > 0 {
		last := history[len(history)-1]
		if last.StoppedAt == nil {
			return fmt.Errorf("metrics session already active for cluster %q", clusterID)
		}
	}

	newSession := &Session{
		ClusterID:   clusterID,
		StartedAt:   now,
		trackedKeys: make(map[string]struct{}),
		snapshots:   make(map[string][]Snapshot),
		timeline:    make(map[string][]TimelineEvent),
	}

	// Limit history to 50 sessions
	if len(history) >= 50 {
		history = history[1:]
	}
	m.sessions[clusterID] = append(history, newSession)

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

	history := m.sessions[clusterID]
	if len(history) == 0 {
		return fmt.Errorf("no metrics session for cluster %q", clusterID)
	}

	s := history[len(history)-1]
	if s.StoppedAt != nil {
		return fmt.Errorf("metrics session already stopped for cluster %q", clusterID)
	}

	stopped := now
	s.StoppedAt = &stopped
	return nil
}

// UpdateSessionStats updates the stats for the current session.
func (m *SessionManager) UpdateSessionStats(clusterID string, stats SessionStats) {
	m.mu.Lock()
	defer m.mu.Unlock()
	history := m.sessions[clusterID]
	if len(history) > 0 {
		history[len(history)-1].Stats = stats
	}
}

// IsActive reports whether a cluster has an active metrics session.
func (m *SessionManager) IsActive(clusterID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	history := m.sessions[strings.TrimSpace(clusterID)]
	if len(history) == 0 {
		return false
	}
	return history[len(history)-1].StoppedAt == nil
}

// TrackKey registers a key for metrics collection under an active session.
func (m *SessionManager) TrackKey(clusterID, key string) error {
	clusterID = strings.TrimSpace(clusterID)
	key = strings.TrimSpace(key)

	m.mu.Lock()
	defer m.mu.Unlock()

	history := m.sessions[clusterID]
	if len(history) == 0 {
		return fmt.Errorf("no metrics session for cluster %q", clusterID)
	}

	s := history[len(history)-1]
	if s.StoppedAt != nil {
		return fmt.Errorf("metrics session is stopped for cluster %q", clusterID)
	}

	s.trackedKeys[key] = struct{}{}
	if _, exists := s.snapshots[key]; !exists {
		s.snapshots[key] = []Snapshot{}
	}
	if _, exists := s.timeline[key]; !exists {
		s.timeline[key] = []TimelineEvent{}
	}
	return nil
}

// RecordTimelineEvent appends one fine-grained event into the active session.
func (m *SessionManager) RecordTimelineEvent(clusterID, key string, observedAt time.Time, event TimelineEvent) error {
	clusterID = strings.TrimSpace(clusterID)
	key = strings.TrimSpace(key)

	m.mu.Lock()
	defer m.mu.Unlock()

	history := m.sessions[clusterID]
	if len(history) == 0 {
		return fmt.Errorf("no metrics session for cluster %q", clusterID)
	}

	s := history[len(history)-1]
	if s.StoppedAt != nil {
		return fmt.Errorf("metrics session is stopped for cluster %q", clusterID)
	}

	s.trackedKeys[key] = struct{}{}
	if _, exists := s.timeline[key]; !exists {
		s.timeline[key] = []TimelineEvent{}
	}

	t := observedAt.Sub(s.StartedAt)
	if t < 0 {
		t = 0
	}
	event.Time = t

	s.timeline[key] = append(s.timeline[key], event)
	return nil
}

// RecordSnapshot appends one key snapshot into the active session.
func (m *SessionManager) RecordSnapshot(clusterID, key string, observedAt time.Time, nodes map[string]NodeState) error {
	clusterID = strings.TrimSpace(clusterID)
	key = strings.TrimSpace(key)

	m.mu.Lock()
	defer m.mu.Unlock()

	history := m.sessions[clusterID]
	if len(history) == 0 {
		return fmt.Errorf("no metrics session for cluster %q", clusterID)
	}

	s := history[len(history)-1]
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

// GetSession returns a read-only snapshot of the latest session.
func (m *SessionManager) GetSession(clusterID string) (SessionSnapshot, error) {
	clusterID = strings.TrimSpace(clusterID)

	m.mu.RLock()
	defer m.mu.RUnlock()

	history := m.sessions[clusterID]
	if len(history) == 0 {
		return SessionSnapshot{}, fmt.Errorf("no metrics session for cluster %q", clusterID)
	}

	s := history[len(history)-1]

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
		Stats:       s.Stats,
	}, nil
}

// GetHistory returns all stored sessions for a cluster.
func (m *SessionManager) GetHistory(clusterID string) []SessionSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	history := m.sessions[clusterID]
	snaps := make([]SessionSnapshot, len(history))
	for i, s := range history {
		keys := make([]string, 0, len(s.trackedKeys))
		for k := range s.trackedKeys {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		var stopped *time.Time
		if s.StoppedAt != nil {
			copyStopped := *s.StoppedAt
			stopped = &copyStopped
		}

		snaps[i] = SessionSnapshot{
			ClusterID:   s.ClusterID,
			StartedAt:   s.StartedAt,
			StoppedAt:   stopped,
			TrackedKeys: keys,
			Stats:       s.Stats,
		}
	}
	return snaps
}

// ComputeKeyResult computes metrics for a single key in the latest session.
func (m *SessionManager) ComputeKeyResult(clusterID, key string) (Result, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	history, ok := m.sessions[clusterID]
	if !ok || len(history) == 0 {
		return Result{}, fmt.Errorf("no metrics session for cluster %q", clusterID)
	}

	s := history[len(history)-1]
	res := Compute(s.snapshots[key])
	res.Timeline = s.timeline[key]
	res.ConvergenceCurve = computeConvergenceCurve(s.timeline[key])
	return res, nil
}

// ComputeResultsBySessionID computes results for a specific session in history.
func (m *SessionManager) ComputeResultsBySessionID(clusterID string, sessionIdx int) (map[string]Result, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	history, ok := m.sessions[clusterID]
	if !ok || sessionIdx < 0 || sessionIdx >= len(history) {
		return nil, fmt.Errorf("session not found")
	}

	s := history[sessionIdx]
	out := make(map[string]Result)
	allKeys := make(map[string]struct{})
	for k := range s.snapshots { allKeys[k] = struct{}{} }
	for k := range s.timeline { allKeys[k] = struct{}{} }

	for key := range allKeys {
		res := Compute(s.snapshots[key])
		res.Timeline = s.timeline[key]
		res.ConvergenceCurve = computeConvergenceCurve(s.timeline[key])
		out[key] = res
	}
	return out, nil
}

// ComputeAllResults computes metrics for all tracked keys in the LATEST session.
func (m *SessionManager) ComputeAllResults(clusterID string) (map[string]Result, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	history, ok := m.sessions[clusterID]
	if !ok || len(history) == 0 {
		return nil, fmt.Errorf("no metrics session for cluster %q", clusterID)
	}

	s := history[len(history)-1]

	out := make(map[string]Result, len(s.snapshots))
	// Collect all keys from snapshots and timeline
	allKeys := make(map[string]struct{})
	for k := range s.snapshots { allKeys[k] = struct{}{} }
	for k := range s.timeline { allKeys[k] = struct{}{} }

	for key := range allKeys {
		res := Compute(s.snapshots[key])
		res.Timeline = s.timeline[key]
		res.ConvergenceCurve = computeConvergenceCurve(s.timeline[key])
		out[key] = res
	}
	return out, nil
}

func computeConvergenceCurve(events []TimelineEvent) []DivergencePoint {
	if len(events) == 0 {
		return nil
	}
	
	// Track current value per node
	nodeValues := make(map[string]string)
	var curve []DivergencePoint
	
	// Sort events by time just in case
	sorted := make([]TimelineEvent, len(events))
	copy(sorted, events)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Time < sorted[j].Time
	})
	
	for _, e := range sorted {
		nodeValues[e.NodeID] = e.Value + "|" + e.Origin + "|" + fmt.Sprint(e.Version)
		
		// Count distinct values
		distinct := make(map[string]struct{})
		for _, v := range nodeValues {
			distinct[v] = struct{}{}
		}
		
		curve = append(curve, DivergencePoint{
			Time:       e.Time,
			Divergence: len(distinct),
		})
	}
	return curve
}
