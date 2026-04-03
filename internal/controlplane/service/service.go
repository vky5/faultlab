package service

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/vky5/faultlab/internal/cluster"
	clustermanager "github.com/vky5/faultlab/internal/cluster/manager"
	"github.com/vky5/faultlab/internal/protocol"
	gproto "google.golang.org/protobuf/proto"
)

type Service struct {
	cluster *clustermanager.Manager
	// NodeClient *controlplanerpc.NodeClient // ? this is hard dependency and it exposes the APIs of the Proto directly to service, we just need to call the function that's why this abstraction is necessary
	NodeClient NodeOperator

	logMu        sync.RWMutex
	logListeners map[chan *protocol.LogRequest]struct{}
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

func (s *Service) Heartbeat(clusterID, nodeID string) error {
	return s.cluster.Heartbeat(clusterID, nodeID)
}

func (s *Service) SetFaultParams(clusterID, nodeID string, fault cluster.FaultState) error {
	return s.cluster.SetFaultParams(clusterID, nodeID, fault)
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

func (s *Service) ExecuteKVGet(ctx context.Context, clusterID, nodeID, key string) (string, error) {
	n, err := s.cluster.GetNode(clusterID, nodeID)
	if err != nil {
		return "", err
	}

	payload, err := gproto.Marshal(&protocol.KVGetRequest{Key: key})
	if err != nil {
		return "", fmt.Errorf("marshal kv-get payload: %w", err)
	}

	resp, err := s.NodeClient.ExecuteAction(ctx, n.Address, n.Port, &protocol.ActionRequest{
		NodeId:  nodeID,
		Action:  protocol.ActionType_KV_GET,
		Payload: payload,
	})
	if err != nil {
		return "", fmt.Errorf("execute kv-get failed: %w", err)
	}

	if !resp.GetSuccess() {
		return "", fmt.Errorf("kv-get rejected: %s", resp.GetMessage())
	}

	var out protocol.KVGetResponse
	if err := gproto.Unmarshal(resp.GetPayload(), &out); err != nil {
		return "", fmt.Errorf("decode kv-get response: %w", err)
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

	return out.GetValue(), nil
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
