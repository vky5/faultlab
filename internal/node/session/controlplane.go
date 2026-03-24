package session

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/vky5/faultlab/internal/node/runtime"
	"github.com/vky5/faultlab/internal/protocol"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type cpsession struct {
	cpHost string
	cpPort int

	nodeID    string
	clusterID string
	nodeHost  string
	nodePort  int

	mu     sync.Mutex
	conn   *grpc.ClientConn
	client protocol.OrchestratorServiceClient
}

func NewControlplaneSession(
	nodeID string,
	clusterID string,
	cpHost string,
	cpPort int,
	nodeHost string,
	nodePort int,
) runtime.CPSession {
	return &cpsession{
		nodeID:    nodeID,
		clusterID: clusterID,
		cpHost:    cpHost,
		cpPort:    cpPort,
		nodeHost:  nodeHost,
		nodePort:  nodePort,
	}
}

func (s *cpsession) connect(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn != nil {
		return nil
	}

	addr := fmt.Sprintf("%s:%d", s.cpHost, s.cpPort)

	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return err
	}

	s.conn = conn
	s.client = protocol.NewOrchestratorServiceClient(conn)

	return nil
}

func (s *cpsession) invalidateConn() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn != nil {
		_ = s.conn.Close()
		s.conn = nil
		s.client = nil
	}
}

func (s *cpsession) Establish(ctx context.Context) error {
	if err := s.connect(ctx); err != nil {
		return err
	}

	opCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	resp, err := s.client.RegisterNode(opCtx, &protocol.RegisterNodeRequest{
		ClusterId: s.clusterID,
		NodeId:    s.nodeID,
		Address:   s.nodeHost,
		Port:      int32(s.nodePort),
	})

	if err != nil {
		s.invalidateConn() // mark connection unhealthy
		return err
	}

	if resp.Status != protocol.RegisterStatus_SUCCESS {
		return fmt.Errorf("registration rejected: %s", resp.Message)
	}

	return nil
}

// Heartbeat sends a heartbeat to the control plane
func (s *cpsession) Heartbeat(ctx context.Context) error {
	if err := s.connect(ctx); err != nil {
		return err
	}

	opCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	resp, err := s.client.Heartbeat(opCtx, &protocol.HeartbeatRequest{
		Id:        s.nodeID,
		ClusterId: s.clusterID,
	})
	if err != nil {
		s.invalidateConn()
		return err
	}

	if !resp.Ok {
		return fmt.Errorf("heartbeat rejected by controlplane")
	}

	return nil
}

// FetchPeers fetches the peer list from the control plane
func (s *cpsession) FetchPeers(ctx context.Context) ([]*protocol.NodeInfo, error) {
	if err := s.connect(ctx); err != nil {
		return nil, err
	}

	opCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	resp, err := s.client.GetPeers(opCtx, &protocol.PeersRequest{
		ClusterId: s.clusterID,
	})
	if err != nil {
		s.invalidateConn()
		return nil, err
	}

	return resp.Peers, nil
}

// ReportLog streams a log entry to the control plane
func (s *cpsession) ReportLog(ctx context.Context, level, msg string) error {
	if err := s.connect(ctx); err != nil {
		return err
	}

	opCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	_, err := s.client.ReportLog(opCtx, &protocol.LogRequest{
		ClusterId: s.clusterID,
		NodeId:    s.nodeID,
		Timestamp: time.Now().UnixMilli(),
		Level:     level,
		Message:   msg,
	})
	
	if err != nil {
		// we don't invalidate connection violently for dropped logs
		return err
	}
	return nil
}
