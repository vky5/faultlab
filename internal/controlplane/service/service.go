package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
	"strconv"

	clustermanager "github.com/vky5/faultlab/internal/cluster/manager"
	"github.com/vky5/faultlab/internal/metrics"
	"github.com/vky5/faultlab/internal/protocol"
	gproto "google.golang.org/protobuf/proto"
	"github.com/vky5/faultlab/internal/cluster"
)

type Service struct {
	cluster *clustermanager.Manager
	NodeClient NodeOperator

	logMu        sync.RWMutex
	logListeners map[chan *protocol.LogRequest]struct{}
	metrics      *metrics.SessionManager
	metricsMu    sync.Mutex
	metricsLoops map[string]context.CancelFunc

	statsMu     sync.Mutex
	totalRPCs   map[string]uint64
	totalWrites map[string]uint64
	recentKeys  map[string][]string

	sessionBaselines map[string]struct{ rpcs, writes uint64 }
}

type KVGetResult struct {
	Value       string
	Version     int64
	Origin      string
	HasMetadata bool
}

func NewClusterService(cluster *clustermanager.Manager, nodeClient NodeOperator) *Service {
	return &Service{
		cluster:      cluster,
		NodeClient:   nodeClient,
		logListeners: make(map[chan *protocol.LogRequest]struct{}),
		metrics:      metrics.NewSessionManager(),
		metricsLoops: make(map[string]context.CancelFunc),
		totalRPCs:     make(map[string]uint64),
		totalWrites:   make(map[string]uint64),
		recentKeys:    make(map[string][]string),
		sessionBaselines: make(map[string]struct{ rpcs, writes uint64 }),
	}
}

func (s *Service) Metrics() *metrics.SessionManager {
	return s.metrics
}

func (s *Service) StartMetricsSession(clusterID string, interval time.Duration) error {
	clusterID = strings.TrimSpace(clusterID)
	if clusterID == "" {
		return fmt.Errorf("cluster id is required")
	}
	if interval <= 0 {
		interval = time.Second
	}

	if _, err := s.cluster.GetCluster(clusterID); err != nil {
		return err
	}

	if err := s.metrics.Start(clusterID, time.Now()); err != nil {
		return err
	}

	s.statsMu.Lock()
	s.sessionBaselines[clusterID] = struct{ rpcs, writes uint64 }{
		rpcs:   s.totalRPCs[clusterID],
		writes: s.totalWrites[clusterID],
	}
	s.statsMu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	s.metricsMu.Lock()
	s.metricsLoops[clusterID] = cancel
	s.metricsMu.Unlock()

	go s.runMetricsSampler(ctx, clusterID, interval)
	return nil
}

func (s *Service) StartMetricsSessionIfInactive(clusterID string, interval time.Duration) (bool, error) {
	if s.metrics.IsActive(clusterID) {
		return false, nil
	}
	if err := s.StartMetricsSession(clusterID, interval); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Service) StopMetricsSession(clusterID string) (map[string]any, error) {
	clusterID = strings.TrimSpace(clusterID)

	// Capture final state
	s.sampleAllTrackedKeys(context.Background(), clusterID)

	s.metricsMu.Lock()
	if cancel, ok := s.metricsLoops[clusterID]; ok {
		cancel()
		delete(s.metricsLoops, clusterID)
	}
	s.metricsMu.Unlock()

	// Capture final stats BEFORE stopping baseline tracking
	s.statsMu.Lock()
	baseline := s.sessionBaselines[clusterID]
	finalStats := metrics.SessionStats{
		TotalRPCs:   s.totalRPCs[clusterID] - baseline.rpcs,
		TotalWrites: s.totalWrites[clusterID] - baseline.writes,
		RecentKeys:  s.recentKeys[clusterID],
	}
	delete(s.sessionBaselines, clusterID)
	s.statsMu.Unlock()

	// Update the session in SessionManager with sealed stats
	s.metrics.UpdateSessionStats(clusterID, finalStats)

	if err := s.metrics.Stop(clusterID, time.Now()); err != nil {
		return nil, err
	}

	return s.GetMetricsSnapshot(clusterID)
}

func (s *Service) StopMetricsSessionIfActive(clusterID string) {
	if s.metrics.IsActive(clusterID) {
		s.StopMetricsSession(clusterID)
	}
}

// WaitForConvergence blocks until all nodes have reached an identical state for all tracked keys, 
// or until the timeout is reached. Returns true if convergence was reached.
func (s *Service) WaitForConvergence(clusterID string, timeout time.Duration) bool {
	start := time.Now()
	for time.Since(start) < timeout {
		converged, err := s.IsClusterConverged(clusterID)
		if err == nil && converged {
			return true
		}
		time.Sleep(1 * time.Second)
	}
	return false
}

// IsClusterConverged performs a point-in-time check across all nodes to see if 
// they agree on all currently tracked keys in the active metrics session.
func (s *Service) IsClusterConverged(clusterID string) (bool, error) {
	session, err := s.metrics.GetSession(clusterID)
	if err != nil {
		return false, err
	}
	if len(session.TrackedKeys) == 0 {
		return true, nil
	}

	nodes, err := s.cluster.GetNodes(clusterID)
	if err != nil || len(nodes) == 0 {
		return false, err
	}

	for _, key := range session.TrackedKeys {
		var first metrics.NodeState
		var firstSet bool

		for _, n := range nodes {
			ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
			res, found, err := s.ExecuteKVGetObserved(ctx, clusterID, n.ID, key)
			cancel()

			if err != nil {
				return false, fmt.Errorf("convergence check failed for node %s key %s: %w", n.ID, key, err)
			}

			state := metrics.NodeState{Exists: found}
			if found {
				state.Value = res.Value
				state.Version = res.Version
				state.Origin = res.Origin
				state.HasMetadata = res.HasMetadata
			}

			if !firstSet {
				first = state
				firstSet = true
				if !state.Exists {
					// If the first node doesn't have it, we are not converged (at least one node must have it if we are tracking it)
					return false, nil
				}
				continue
			}

			if state != first {
				return false, nil
			}
		}
	}

	return true, nil
}

func (s *Service) AddMetricsWatchKey(clusterID, key string) error {
	return s.metrics.TrackKey(clusterID, key)
}

func (s *Service) RecordWriteKeyForMetrics(clusterID, key string) {
	if s.metrics.IsActive(clusterID) {
		s.metrics.TrackKey(clusterID, key)
	}
}

func (s *Service) GetMetricsSnapshot(clusterID string) (map[string]any, error) {
	clusterID = strings.TrimSpace(clusterID)
	snap, err := s.metrics.GetSession(clusterID)
	if err != nil {
		return nil, err
	}

	results, err := s.metrics.ComputeAllResults(clusterID)
	if err != nil {
		return nil, err
	}

	s.statsMu.Lock()
	baseline := s.sessionBaselines[clusterID]
	stats := map[string]any{
		"totalRPCs":   s.totalRPCs[clusterID] - baseline.rpcs,
		"totalWrites": s.totalWrites[clusterID] - baseline.writes,
		"recentKeys":  s.recentKeys[clusterID],
	}
	s.statsMu.Unlock()

	return map[string]any{
		"isActive":     s.metrics.IsActive(clusterID),
		"clusterId":    clusterID,
		"startedAt":    snap.StartedAt,
		"stoppedAt":    snap.StoppedAt,
		"trackedKeys":  snap.TrackedKeys,
		"results":      results,
		"clusterStats": stats,
	}, nil
}

func (s *Service) GetMetricsHistory(clusterID string) ([]map[string]any, error) {
	clusterID = strings.TrimSpace(clusterID)
	history := s.metrics.GetHistory(clusterID)
	res := make([]map[string]any, 0, len(history))

	for i, snap := range history {
		results, err := s.metrics.ComputeResultsBySessionID(clusterID, i)
		if err != nil {
			continue
		}

		res = append(res, map[string]any{
			"clusterId":    snap.ClusterID,
			"startedAt":    snap.StartedAt,
			"stoppedAt":    snap.StoppedAt,
			"trackedKeys":  snap.TrackedKeys,
			"results":      results,
			"isActive":     snap.StoppedAt == nil,
			"clusterStats": map[string]any{
				"totalRPCs":   snap.Stats.TotalRPCs,
				"totalWrites": snap.Stats.TotalWrites,
				"recentKeys":  snap.Stats.RecentKeys,
			},
		})
	}

	return res, nil
}

func (s *Service) runMetricsSampler(ctx context.Context, clusterID string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	s.sampleAllTrackedKeys(ctx, clusterID)
	for {
		select {
		case <-ctx.Done(): return
		case <-ticker.C: s.sampleAllTrackedKeys(ctx, clusterID)
		}
	}
}

func (s *Service) sampleAllTrackedKeys(ctx context.Context, clusterID string) {
	if !s.metrics.IsActive(clusterID) { return }
	session, err := s.metrics.GetSession(clusterID)
	if err != nil || len(session.TrackedKeys) == 0 { return }

	nodes, err := s.cluster.GetNodes(clusterID)
	if err != nil || len(nodes) == 0 { return }

	now := time.Now()
	for _, key := range session.TrackedKeys {
		nodeStates := make(map[string]metrics.NodeState, len(nodes))
		keyFailed := false
		for _, n := range nodes {
			queryCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
			res, found, err := s.ExecuteKVGetObserved(queryCtx, clusterID, n.ID, key)
			cancel()
			if err != nil {
				keyFailed = true
				break
			}
			if !found {
				nodeStates[n.ID] = metrics.NodeState{Exists: false}
				continue
			}
			nodeStates[n.ID] = metrics.NodeState{
				Exists:      true,
				Value:       res.Value,
				Version:     res.Version,
				Origin:      res.Origin,
				HasMetadata: res.HasMetadata,
			}
		}
		if !keyFailed {
			s.metrics.RecordSnapshot(clusterID, key, now, nodeStates)
		}
	}
}

func (s *Service) ExecuteKVGetObserved(ctx context.Context, clusterID, nodeID, key string) (KVGetResult, bool, error) {
	n, err := s.cluster.GetNode(clusterID, nodeID)
	if err != nil { return KVGetResult{}, false, err }
	payload, _ := gproto.Marshal(&protocol.KVGetRequest{Key: key})
	resp, err := s.NodeClient.ExecuteAction(ctx, n.Address, n.Port, &protocol.ActionRequest{
		NodeId:  nodeID,
		Action:  protocol.ActionType_KV_GET,
		Payload: payload,
	})
	if err != nil { return KVGetResult{}, false, err }
	if !resp.GetSuccess() { return KVGetResult{}, false, nil }
	var out protocol.KVGetResponse
	gproto.Unmarshal(resp.GetPayload(), &out)
	return KVGetResult{
		Value: out.GetValue(), Version: out.GetVersion(), Origin: out.GetOrigin(), HasMetadata: out.GetHasMetadata(),
	}, true, nil
}

func (s *Service) BroadcastLog(req *protocol.LogRequest) {
	if req != nil {
		if strings.Contains(req.Message, "TRACE:SEND:") {
			s.statsMu.Lock()
			s.totalRPCs[req.ClusterId]++
			s.statsMu.Unlock()
		}
		if strings.HasPrefix(req.Message, "TL_EVENT:") {
			s.handleTimelineLog(req)
		}
	}
	s.logMu.RLock()
	defer s.logMu.RUnlock()
	for ch := range s.logListeners {
		select {
		case ch <- req:
		default:
		}
	}
}

func (s *Service) handleTimelineLog(req *protocol.LogRequest) {
	// Format: TL_EVENT:<TYPE>|<KEY>|<VALUE>|<VERSION>|<ORIGIN>|<SOURCE>
	raw := strings.TrimPrefix(req.Message, "TL_EVENT:")
	parts := strings.Split(raw, "|")
	if len(parts) < 6 {
		return
	}
	
	version, _ := strconv.ParseInt(parts[3], 10, 64)
	var round uint64
	var lwwTs int64
	if len(parts) > 6 {
		r, _ := strconv.ParseUint(parts[6], 10, 64)
		round = r
	}
	if len(parts) > 7 {
		lt, _ := strconv.ParseInt(parts[7], 10, 64)
		lwwTs = lt
	}

	event := metrics.TimelineEvent{
		NodeID:       req.NodeId,
		Key:          parts[1],
		Value:        parts[2],
		Version:      version,
		Origin:       parts[4],
		Source:       parts[5],
		EventType:    parts[0],
		Round:        round,
		LWWTimestamp: lwwTs,
	}
	
	// Use req.Timestamp as wall clock if provided
	ts := time.Now()
	if req.Timestamp > 0 {
		ts = time.UnixMilli(req.Timestamp)
	}

	s.metrics.RecordTimelineEvent(req.ClusterId, parts[1], ts, event)
}

func (s *Service) ExecuteKVPut(ctx context.Context, clusterID, nodeID, key, value string) error {
	n, err := s.cluster.GetNode(clusterID, nodeID)
	if err != nil { return err }
	payload, _ := gproto.Marshal(&protocol.KVPutRequest{Key: key, Value: value})
	resp, err := s.NodeClient.ExecuteAction(ctx, n.Address, n.Port, &protocol.ActionRequest{
		NodeId:  nodeID,
		Action:  protocol.ActionType_KV_PUT,
		Payload: payload,
	})
	if err != nil || !resp.GetSuccess() { return err }

	s.statsMu.Lock()
	s.totalWrites[clusterID]++
	seen := false
	for _, k := range s.recentKeys[clusterID] {
		if k == key { seen = true; break }
	}
	if !seen {
		s.recentKeys[clusterID] = append([]string{key}, s.recentKeys[clusterID]...)
		if len(s.recentKeys[clusterID]) > 10 { s.recentKeys[clusterID] = s.recentKeys[clusterID][:10] }
	}
	s.statsMu.Unlock()
	return nil
}

func (s *Service) ExecuteKVGet(ctx context.Context, clusterID, nodeID, key string) (KVGetResult, error) {
	res, found, err := s.ExecuteKVGetObserved(ctx, clusterID, nodeID, key)
	if err != nil { return KVGetResult{}, err }
	if !found { return KVGetResult{}, fmt.Errorf("not found") }
	return res, nil
}

func (s *Service) GetPeers(clusterID string) ([]cluster.Node, error) { return s.cluster.GetNodes(clusterID) }
func (s *Service) CreateCluster(clusterID, protocol string) error {
	err := s.cluster.CreateCluster(clusterID, protocol)
	if err == nil {
		s.RecordLifecycleEvent(clusterID, "SYSTEM", "CLUSTER_CREATE", fmt.Sprintf("new cluster created with protocol: %s", protocol))
	}
	return err
}
func (s *Service) GetClusters() []string { return s.cluster.GetClusters() }
func (s *Service) GetCluster(clusterID string) (*cluster.Cluster, error) { return s.cluster.GetCluster(clusterID) }
func (s *Service) RemoveCluster(clusterID string) error { return s.cluster.RemoveCluster(clusterID) }
func (s *Service) GetNodes(clusterID string) ([]cluster.Node, error) { return s.cluster.GetNodes(clusterID) }
func (s *Service) GetNode(clusterID, nodeID string) (*cluster.Node, error) { return s.cluster.GetNode(clusterID, nodeID) }
func (s *Service) GetClusterProtocol(clusterID string) (string, error) {
	c, err := s.cluster.GetCluster(clusterID)
	if err != nil { return "", err }
	if c.Protocol == "" { return "gossip", nil }
	return c.Protocol, nil
}
func (s *Service) SetClusterProtocol(ctx context.Context, clusterID, protocolKey string) error {
	protocolKey = strings.ToLower(strings.TrimSpace(protocolKey))
	nodes, err := s.cluster.GetNodes(clusterID)
	if err != nil { return err }
	for _, n := range nodes {
		s.NodeClient.SwapProtocol(ctx, n.Address, n.Port, clusterID, n.ID, protocolKey, 0)
	}
	return s.cluster.SwapProtocol(clusterID, protocolKey)
}
func (s *Service) Heartbeat(clusterID, nodeID string) error { return s.cluster.Heartbeat(clusterID, nodeID) }
func (s *Service) SetFaultParams(clusterID, nodeID string, fault cluster.FaultState) error {
	// Detect changes for lifecycle logging
	oldNode, _ := s.cluster.GetNode(clusterID, nodeID)
	
	err := s.cluster.SetFaultParams(clusterID, nodeID, fault)
	if err != nil { return err }

	if oldNode != nil {
		if !oldNode.Fault.Crashed && fault.Crashed {
			s.RecordLifecycleEvent(clusterID, nodeID, "CRASH", "node crashed")
		} else if oldNode.Fault.Crashed && !fault.Crashed {
			s.RecordLifecycleEvent(clusterID, nodeID, "RECOVER", "node recovered")
		}
		
		// For partitions, it's more complex since it's a list.
		// We'll just log that fault parameters were updated for now, 
		// or specifically check partition count changes.
		if len(oldNode.Fault.Partition) < len(fault.Partition) {
			s.RecordLifecycleEvent(clusterID, nodeID, "PARTITION", "new network partition applied")
		} else if len(oldNode.Fault.Partition) > len(fault.Partition) {
			s.RecordLifecycleEvent(clusterID, nodeID, "HEAL", "network partition healed")
		}
	}

	return nil
}
func (s *Service) ReportNodeCapabilities(req *protocol.ReportNodeCapabilitiesRequest) error {
	assignment := req.GetActiveProtocol()
	actions := req.GetActions()
	return s.cluster.SetNodeCapabilities(
		req.GetClusterId(), req.GetNodeId(), assignment.GetKey(), assignment.GetEpoch(),
		cluster.NodeActionCapabilities{KVPut: actions.GetKvPut(), KVGet: actions.GetKvGet(), KVDelete: actions.GetKvDelete()},
		req.GetReportedAt(),
	)
}
func (s *Service) SubscribeLogs() chan *protocol.LogRequest {
	ch := make(chan *protocol.LogRequest, 100)
	s.logMu.Lock(); s.logListeners[ch] = struct{}{}; s.logMu.Unlock()
	return ch
}
func (s *Service) UnsubscribeLogs(ch chan *protocol.LogRequest) {
	s.logMu.Lock(); delete(s.logListeners, ch); close(ch); s.logMu.Unlock()
}
func (s *Service) RemoveNodeC(clusterID, nodeID string) error { return s.cluster.RemoveNode(clusterID, nodeID) }
func (s *Service) RemoveNode(ctx context.Context, clusterID, nodeID string) error {
	n, _ := s.cluster.GetNode(clusterID, nodeID)
	s.NodeClient.StopNode(ctx, n.Address, n.Port)
	return s.cluster.RemoveNode(clusterID, nodeID)
}
func (s *Service) RegisterNode(ctx context.Context, clusterID, nodeID, host string, port int) error {
	s.NodeClient.Ping(ctx, host, port)
	s.cluster.RegisterNode(clusterID, nodeID, host, port)
	s.RecordLifecycleEvent(clusterID, nodeID, "NODE_JOIN", fmt.Sprintf("node joined cluster at %s:%d", host, port))
	return nil
}

func (s *Service) RecordLifecycleEvent(clusterID, nodeID, eventType, details string) {
	if !s.metrics.IsActive(clusterID) {
		return
	}
	s.metrics.RecordTimelineEvent(clusterID, "__CLUSTER__", time.Now(), metrics.TimelineEvent{
		NodeID:    nodeID,
		EventType: eventType,
		Value:     details,
		Key:       "__CLUSTER__",
	})
}
