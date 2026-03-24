package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/vky5/faultlab/internal/cluster"
	clustermanager "github.com/vky5/faultlab/internal/cluster/manager"
)

type Service struct {
	cluster *clustermanager.Manager
	// NodeClient *controlplanerpc.NodeClient // ? this is hard dependency and it exposes the APIs of the Proto directly to service, we just need to call the function that's why this abstraction is necessary
	NodeClient NodeOperator
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
		cluster:    cluster,
		NodeClient: nodeClient,
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

	return nil
}

func (s *Service) GetPeers(clusterID string) ([]cluster.Node, error) {
	return s.cluster.GetNodes(clusterID)
}

func (s *Service) Heartbeat(clusterID, nodeID string) error {
	return s.cluster.Heartbeat(clusterID, nodeID)
}

func (s *Service) SetFaultParams(clusterID, nodeID string, fault cluster.FaultState) error {
	return s.cluster.SetFaultParams(clusterID, nodeID, fault)
}
