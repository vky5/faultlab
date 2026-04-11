package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/vky5/faultlab/internal/cluster"
	clustermanager "github.com/vky5/faultlab/internal/cluster/manager"
	"github.com/vky5/faultlab/internal/metrics"
	"github.com/vky5/faultlab/internal/protocol"
	gproto "google.golang.org/protobuf/proto"
)

type Service struct {
	cluster *clustermanager.Manager
	// NodeClient *controlplanerpc.NodeClient // ? this is hard dependency and it exposes the APIs of the Proto directly to service, we just need to call the function that's why this abstraction is necessary
	NodeClient NodeOperator

	logMu        sync.RWMutex
	logListeners map[chan *protocol.LogRequest]struct{}
	metrics      *metrics.SessionManager
	metricsMu    sync.Mutex
	metricsLoops map[string]context.CancelFunc
}

type KVGetResult struct {
	Value       string
	Version     int64
	Origin      string
	HasMetadata bool
}

/*
CLI / RPC / API
        ↓
Controlplane Service  ← brain
        ↓
Cluster Manager       ← memory
*/

func NewClusterService(cluster *clustermanager.Manager, nodeClient NodeOperator) *Service {
	return &Service{
		cluster:      cluster,
		NodeClient:   nodeClient,
		logListeners: make(map[chan *protocol.LogRequest]struct{}),
		metrics:      metrics.NewSessionManager(),
		metricsLoops: make(map[string]context.CancelFunc),
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

	ctx, cancel := context.WithCancel(context.Background())
	s.metricsMu.Lock()
	s.metricsLoops[clusterID] = cancel
	s.metricsMu.Unlock()

	go s.runMetricsSampler(ctx, clusterID, interval)
	return nil
}

func (s *Service) StartMetricsSessionIfInactive(clusterID string, interval time.Duration) (bool, error) {
	clusterID = strings.TrimSpace(clusterID)
	if clusterID == "" {
		return false, fmt.Errorf("cluster id is required")
	}
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
	if clusterID == "" {
		return nil, fmt.Errorf("cluster id is required")
	}

	s.metricsMu.Lock()
	if cancel, ok := s.metricsLoops[clusterID]; ok {
		cancel()
		delete(s.metricsLoops, clusterID)
	}
	s.metricsMu.Unlock()

	if err := s.metrics.Stop(clusterID, time.Now()); err != nil {
		return nil, err
	}

	snap, err := s.metrics.GetSession(clusterID)
	if err != nil {
		return nil, err
	}
	results, err := s.metrics.ComputeAllResults(clusterID)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"status":      "stopped",
		"clusterId":   clusterID,
		"startedAt":   snap.StartedAt,
		"stoppedAt":   snap.StoppedAt,
		"trackedKeys": snap.TrackedKeys,
		"results":     results,
	}, nil
}

func (s *Service) StopMetricsSessionIfActive(clusterID string) {
	clusterID = strings.TrimSpace(clusterID)
	if clusterID == "" {
		return
	}
	if !s.metrics.IsActive(clusterID) {
		return
	}
	if _, err := s.StopMetricsSession(clusterID); err != nil {
		log.Printf("stop metrics session during cleanup failed: cluster=%s err=%v", clusterID, err)
	}
}

func (s *Service) AddMetricsWatchKey(clusterID, key string) error {
	return s.metrics.TrackKey(clusterID, key)
}

func (s *Service) RecordWriteKeyForMetrics(clusterID, key string) {
	if !s.metrics.IsActive(clusterID) {
		return
	}
	if err := s.metrics.TrackKey(clusterID, key); err != nil {
		log.Printf("metrics track key skipped: cluster=%s key=%s err=%v", clusterID, key, err)
	}
}

func (s *Service) GetMetricsSnapshot(clusterID string) (map[string]any, error) {
	clusterID = strings.TrimSpace(clusterID)
	if clusterID == "" {
		return nil, fmt.Errorf("cluster id is required")
	}

	snap, err := s.metrics.GetSession(clusterID)
	if err != nil {
		return nil, err
	}

	results, err := s.metrics.ComputeAllResults(clusterID)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"clusterId":   clusterID,
		"startedAt":   snap.StartedAt,
		"stoppedAt":   snap.StoppedAt,
		"trackedKeys": snap.TrackedKeys,
		"results":     results,
	}, nil
}

func (s *Service) runMetricsSampler(ctx context.Context, clusterID string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Try one immediate sample so sessions do not miss early state transitions.
	s.sampleAllTrackedKeys(ctx, clusterID)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sampleAllTrackedKeys(ctx, clusterID)
		}
	}
}

func (s *Service) sampleAllTrackedKeys(ctx context.Context, clusterID string) {
	if !s.metrics.IsActive(clusterID) {
		return
	}

	session, err := s.metrics.GetSession(clusterID)
	if err != nil {
		return
	}
	if len(session.TrackedKeys) == 0 {
		return
	}

	nodes, err := s.cluster.GetNodes(clusterID)
	if err != nil {
		log.Printf("metrics sampler failed to list nodes: cluster=%s err=%v", clusterID, err)
		return
	}
	if len(nodes) == 0 {
		return
	}

	now := time.Now()
	for _, key := range session.TrackedKeys {
		nodeStates := make(map[string]metrics.NodeState, len(nodes))
		ok := true

		for _, n := range nodes {
			queryCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
			res, err := s.ExecuteKVGet(queryCtx, clusterID, n.ID, key)
			cancel()
			if err != nil {
				ok = false
				log.Printf("metrics sampler kv-get failed: cluster=%s key=%s node=%s err=%v", clusterID, key, n.ID, err)
				break
			}

			nodeStates[n.ID] = metrics.NodeState{
				Value:       res.Value,
				Version:     res.Version,
				Origin:      res.Origin,
				HasMetadata: res.HasMetadata,
			}
		}

		if !ok {
			continue
		}
		if err := s.metrics.RecordSnapshot(clusterID, key, now, nodeStates); err != nil {
			log.Printf("metrics sampler failed to record snapshot: cluster=%s key=%s err=%v", clusterID, key, err)
		}
	}
}

// Remove node using
func (s *Service) RemoveNodeC(clusterID, nodeID string) error {
	return s.cluster.RemoveNode(clusterID, nodeID)
}

// Removing Node
func (s *Service) RemoveNode(
	ctx context.Context,
	clusterID, nodeID string,
) error {
	log.Printf("remove node request: cluster=%s node=%s", clusterID, nodeID)

	n, err := s.cluster.GetNode(clusterID, nodeID)
	if err != nil {
		return err
	}

	// stop the node process and kill it
	if err := s.NodeClient.StopNode(ctx, n.Address, n.Port); err != nil {
		return err
	}
	log.Printf("stop-node sent: node=%s addr=%s:%d", nodeID, n.Address, n.Port)

	// remove from cluster state
	if err := s.cluster.RemoveNode(clusterID, nodeID); err != nil {
		return err
	}
	log.Printf("node removed from cluster state: cluster=%s node=%s", clusterID, nodeID)

	return nil
}

// registering node
func (s *Service) RegisterNode(
	ctx context.Context,
	clusterID, nodeID, host string,
	port int,
) error {
	// verification policy (reachability + ping)
	verifyCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	/*
		so we are effectively saying min (ctx, verifyCtx)

		- parent 10s + child 2s → child cancels at 2s
		- parent 1s + child 2s → child cancels at 1s

	*/

	if err := s.NodeClient.Ping(verifyCtx, host, port); err != nil {
		return fmt.Errorf("node verification failed: %w", err)
	}

	// update cluster state
	s.cluster.RegisterNode(clusterID, nodeID, host, port)

	// Visualize registration
	s.BroadcastLog(&protocol.LogRequest{
		ClusterId: clusterID,
		NodeId:    "CP",
		Level:     "INFO",
		Message:   fmt.Sprintf("TRACE:SEND:CP:%s:CP_REGISTRATION", nodeID),
		Timestamp: time.Now().Unix(),
	})

	return nil
}

func (s *Service) GetPeers(clusterID string) ([]cluster.Node, error) {
	return s.cluster.GetNodes(clusterID)
}

func (s *Service) CreateCluster(clusterID, protocol string) error {
	return s.cluster.CreateCluster(clusterID, protocol)
}

func (s *Service) GetClusters() []string {
	return s.cluster.GetClusters()
}

func (s *Service) GetCluster(clusterID string) (*cluster.Cluster, error) {
	return s.cluster.GetCluster(clusterID)
}

func (s *Service) RemoveCluster(clusterID string) error {
	return s.cluster.RemoveCluster(clusterID)
}

func (s *Service) GetNodes(clusterID string) ([]cluster.Node, error) {
	return s.cluster.GetNodes(clusterID)
}

func (s *Service) GetNode(clusterID, nodeID string) (*cluster.Node, error) {
	return s.cluster.GetNode(clusterID, nodeID)
}

func (s *Service) GetClusterProtocol(clusterID string) (string, error) {
	c, err := s.cluster.GetCluster(clusterID)
	if err != nil {
		return "", err
	}

	if c.Protocol == "" {
		return "gossip", nil
	}

	return c.Protocol, nil
}

func (s *Service) SetClusterProtocol(ctx context.Context, clusterID, protocolKey string) error {
	protocolKey = strings.ToLower(strings.TrimSpace(protocolKey))
	if protocolKey == "" {
		protocolKey = "gossip"
	}

	switch protocolKey {
	case "gossip", "raft":
	default:
		return fmt.Errorf("unsupported protocol %q (supported: gossip, raft)", protocolKey)
	}

	nodes, err := s.cluster.GetNodes(clusterID)
	if err != nil {
		return err
	}

	for _, n := range nodes {
		if err := s.NodeClient.SwapProtocol(ctx, n.Address, n.Port, clusterID, n.ID, protocolKey, 0); err != nil {
			return fmt.Errorf("protocol swap failed for node %s: %w", n.ID, err)
		}
	}

	if err := s.cluster.SwapProtocol(clusterID, protocolKey); err != nil {
		return err
	}

	s.BroadcastLog(&protocol.LogRequest{
		ClusterId: clusterID,
		NodeId:    "CP",
		Level:     "INFO",
		Message:   fmt.Sprintf("TRACE:SEND:CP:%s:CP_PROTOCOL_SWAP:%s", clusterID, protocolKey),
		Timestamp: time.Now().Unix(),
	})

	return nil
}

func (s *Service) Heartbeat(clusterID, nodeID string) error {
	return s.cluster.Heartbeat(clusterID, nodeID)
}

func (s *Service) SetFaultParams(clusterID, nodeID string, fault cluster.FaultState) error {
	return s.cluster.SetFaultParams(clusterID, nodeID, fault)
}

func (s *Service) ReportNodeCapabilities(req *protocol.ReportNodeCapabilitiesRequest) error {
	if req == nil {
		return fmt.Errorf("nil capability report request")
	}

	if req.GetClusterId() == "" {
		return fmt.Errorf("cluster_id is required")
	}

	if req.GetNodeId() == "" {
		return fmt.Errorf("node_id is required")
	}

	assignment := req.GetActiveProtocol()
	if assignment == nil {
		return fmt.Errorf("active_protocol is required")
	}

	if assignment.GetKey() == "" {
		return fmt.Errorf("active_protocol.key is required")
	}

	actions := req.GetActions()
	if actions == nil {
		return fmt.Errorf("actions capability contract is required")
	}

	return s.cluster.SetNodeCapabilities(
		req.GetClusterId(),
		req.GetNodeId(),
		assignment.GetKey(),
		assignment.GetEpoch(),
		cluster.NodeActionCapabilities{
			KVPut:    actions.GetKvPut(),
			KVGet:    actions.GetKvGet(),
			KVDelete: actions.GetKvDelete(),
		},
		req.GetReportedAt(),
	)
}

func (s *Service) ExecuteKVPut(ctx context.Context, clusterID, nodeID, key, value string) error {
	n, err := s.cluster.GetNode(clusterID, nodeID)
	if err != nil {
		return err
	}

	payload, err := gproto.Marshal(&protocol.KVPutRequest{Key: key, Value: value})
	if err != nil {
		return fmt.Errorf("marshal kv-put payload: %w", err)
	}

	resp, err := s.NodeClient.ExecuteAction(ctx, n.Address, n.Port, &protocol.ActionRequest{
		NodeId:  nodeID,
		Action:  protocol.ActionType_KV_PUT,
		Payload: payload,
	})
	if err != nil {
		return fmt.Errorf("execute kv-put failed: %w", err)
	}

	if !resp.GetSuccess() {
		return fmt.Errorf("kv-put rejected: %s", resp.GetMessage())
	}

	// Visualize KV Update signaling
	// payload size is apprx len(key) + len(value) + json overhead
	size := len(key) + len(value) + 64
	s.BroadcastLog(&protocol.LogRequest{
		ClusterId: clusterID,
		NodeId:    "CP",
		Level:     "INFO",
		Message:   fmt.Sprintf("TRACE:SEND:CP:%s:CP_KV_PUT:%s=%s:%d", nodeID, key, value, size),
		Timestamp: time.Now().Unix(),
	})

	return nil
}

func (s *Service) ExecuteKVGet(ctx context.Context, clusterID, nodeID, key string) (KVGetResult, error) {
	n, err := s.cluster.GetNode(clusterID, nodeID)
	if err != nil {
		return KVGetResult{}, err
	}

	payload, err := gproto.Marshal(&protocol.KVGetRequest{Key: key})
	if err != nil {
		return KVGetResult{}, fmt.Errorf("marshal kv-get payload: %w", err)
	}

	resp, err := s.NodeClient.ExecuteAction(ctx, n.Address, n.Port, &protocol.ActionRequest{
		NodeId:  nodeID,
		Action:  protocol.ActionType_KV_GET,
		Payload: payload,
	})
	if err != nil {
		return KVGetResult{}, fmt.Errorf("execute kv-get failed: %w", err)
	}

	if !resp.GetSuccess() {
		return KVGetResult{}, fmt.Errorf("kv-get rejected: %s", resp.GetMessage())
	}

	var out protocol.KVGetResponse
	if err := gproto.Unmarshal(resp.GetPayload(), &out); err != nil {
		return KVGetResult{}, fmt.Errorf("decode kv-get response: %w", err)
	}

	// Visualize KV Get signaling
	size := len(key) + 32
	s.BroadcastLog(&protocol.LogRequest{
		ClusterId: clusterID,
		NodeId:    "CP",
		Level:     "INFO",
		Message:   fmt.Sprintf("TRACE:SEND:CP:%s:CP_KV_GET:%s:%d", nodeID, key, size),
		Timestamp: time.Now().Unix(),
	})

	return KVGetResult{
		Value:       out.GetValue(),
		Version:     out.GetVersion(),
		Origin:      out.GetOrigin(),
		HasMetadata: out.GetHasMetadata(),
	}, nil
}

// SubscribeLogs registers a new SSE client for logs
func (s *Service) SubscribeLogs() chan *protocol.LogRequest {
	ch := make(chan *protocol.LogRequest, 100)
	s.logMu.Lock()
	s.logListeners[ch] = struct{}{}
	s.logMu.Unlock()
	return ch
}

// UnsubscribeLogs unregisters an SSE client cleanly
func (s *Service) UnsubscribeLogs(ch chan *protocol.LogRequest) {
	s.logMu.Lock()
	if _, ok := s.logListeners[ch]; ok {
		delete(s.logListeners, ch)
		close(ch)
	}
	s.logMu.Unlock()
}

// BroadcastLog fires a log event to all connected UI clients
func (s *Service) BroadcastLog(req *protocol.LogRequest) {
	s.logMu.RLock()
	defer s.logMu.RUnlock()

	for ch := range s.logListeners {
		select {
		case ch <- req:
		default:
			// slow consumer, drop log to avoid blocking the orchestrator
		}
	}
}
