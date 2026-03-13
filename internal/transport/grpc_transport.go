package transport

import (
	"context"
	"fmt"
	"time"

	"github.com/vky5/faultlab/internal/protocol"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type GRPCTransport struct {
	peers map[string]string // peer id -> Address 
	handler Handler
	timeout time.Duration
}

// Creating new GRPC transport
func NewGRPCTransport(peers map[string]string) *GRPCTransport{
	return &GRPCTransport{
		peers: peers,
		timeout: 2 *time.Second,
	}
}


// registering handler
func (t *GRPCTransport) RegisterHandler(h Handler){
	t.handler = h
}

func (t *GRPCTransport) Send(peerID string, msg Message) error {
	addr, ok := t.peers[peerID]

	if !ok{
		return fmt.Errorf("peer not found: %s", peerID)
	}

	conn, err := grpc.NewClient(
		addr, 
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)

	if err !=nil {
		return fmt.Errorf("failed to establish connection : %s",err)

	}

	defer conn.Close()

	client := protocol.NewNodeServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), t.timeout)
	defer cancel()

	_, err = client.Ping(ctx, &protocol.PingRequest{
		From: msg.From,
	})

	if err!= nil {
		fmt.Errorf("failed to process the request : %s", err)
	}

	return nil
}
