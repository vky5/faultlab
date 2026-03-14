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

type nodeSession struct {
	nodeID   string
	nodePort int
	nodeAddr string

	mu     sync.Mutex
	conn   *grpc.ClientConn
	client protocol.NodeServiceClient
}

func NewNodeSession(
	nodeAddr,
	nodeID string,
	nodePort int,
) runtime.NodeSession {
	return &nodeSession{
		nodeAddr: nodeAddr,
		nodeID:   nodeID,
		nodePort: nodePort,
	}
}

func (ns *nodeSession) connect(ctx context.Context, addr string, port int) (*grpc.ClientConn, protocol.NodeServiceClient, error) {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	if ns.conn != nil {
		return ns.conn, ns.client, nil
	}

	target := fmt.Sprintf("%s:%d", addr, port)

	conn, err := grpc.NewClient(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, nil, err
	}

	ns.conn = conn
	ns.client = protocol.NewNodeServiceClient(conn)

	return conn, ns.client, nil
}

func (ns *nodeSession) invalidateConn() {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	if ns.conn != nil {
		_ = ns.conn.Close()
		ns.conn = nil
		ns.client = nil
	}
}

// Ping sends a ping to a peer node
func (ns *nodeSession) Ping(ctx context.Context, peerID, peerHost string, peerPort int) error {
	_, client, err := ns.connect(ctx, peerHost, peerPort)
	if err != nil {
		return err
	}

	opCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	resp, err := client.Ping(opCtx, &protocol.PingRequest{
		From: ns.nodeID,
	})
	if err != nil {
		ns.invalidateConn()
		return err
	}

	fmt.Printf("%s → ping → %s (%s)\n", ns.nodeID, peerID, resp.Message)
	return nil
}
