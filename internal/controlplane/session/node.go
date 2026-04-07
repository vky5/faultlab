package session

import (
	"context"
	"fmt"
	"time"

	"github.com/vky5/faultlab/internal/cluster"
	"github.com/vky5/faultlab/internal/protocol"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type NodeSession struct {
	timeout time.Duration
}

func NewNodeSession(timeout time.Duration) *NodeSession {
	if timeout <= 0 {
		
		timeout = 3 * time.Second
	}

	return &NodeSession{timeout: timeout}
}

func (s *NodeSession) dial(host string, port int) (*grpc.ClientConn, error) {
	addr := fmt.Sprintf("%s:%d", host, port)

	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("dial failed: %w", err)
	}

	return conn, nil
}

func (s *NodeSession) SetFaultParams(
	ctx context.Context,
	host string,
	port int,
	nodeID string,
	params cluster.FaultState,
) error {
	conn, err := s.dial(host, port)
	if err != nil {
		return err
	}
	defer conn.Close()

	rpcCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	client := protocol.NewNodeServiceClient(conn)
	resp, err := client.SetFaultParams(rpcCtx, &protocol.FaultRequest{
		NodeId:    nodeID,
		Crashed:   params.Crashed,
		DropRate:  params.DropRate,
		DelayMs:   int32(params.DelayMs),
		Partition: params.Partition,
	})
	if err != nil {
		return fmt.Errorf("SetFaultParams RPC failed: %w", err)
	}

	if !resp.GetSuccess() {
		return fmt.Errorf("SetFaultParams rejected: %s", resp.GetMessage())
	}

	return nil
}
