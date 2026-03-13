package service

import (
	"context"
	"log"

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
