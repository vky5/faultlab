package rpc

import (
	"context"
	"fmt"
	"time"

	"github.com/vky5/faultlab/internal/protocol"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type NodeClient struct {
	timeout time.Duration
}

func NewNodeClient(timeout time.Duration) *NodeClient {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}

	return &NodeClient{
		timeout: timeout,
	}
}

// grpc call to stop node grpc server
func (c *NodeClient) StopNode(ctx context.Context, host string, port int) error {
	addr := fmt.Sprintf("%s:%d", host, port)

	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("dial failed: %w", err)
	}
	defer conn.Close()

	rpcCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	client := protocol.NewNodeServiceClient(conn)

	_, err = client.StopNode(rpcCtx, &protocol.RemoveNodeRequest{})
	if err != nil {
		return fmt.Errorf("StopNode RPC failed: %w", err)
	}

	return nil
}

func (n *NodeClient) Ping(ctx context.Context, host string, port int) error {
	addr := fmt.Sprintf("%s:%d", host, port)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}

	defer conn.Close()

	rpcCtx, cancel := context.WithTimeout(ctx, n.timeout)
defer cancel()

	client := protocol.NewNodeServiceClient(conn)

	_, err = client.Ping(rpcCtx, &protocol.PingRequest{From: "Control Plane"})

	return err
}

func (n *NodeClient) ExecuteAction(ctx context.Context, host string, port int, req *protocol.ActionRequest) (*protocol.ActionResponse, error) {
	addr := fmt.Sprintf("%s:%d", host, port)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	rpcCtx, cancel := context.WithTimeout(ctx, n.timeout)
	defer cancel()

	client := protocol.NewNodeServiceClient(conn)

	return client.ExecuteAction(rpcCtx, req)
}
