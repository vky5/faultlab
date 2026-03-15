package session

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/vky5/faultlab/internal/protocol"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type PeerHealth int

const (
	PeerAlive PeerHealth = iota
	PeerSuspect
	PeerDead
)

type grpcConn struct { // storing details related to gRPC connection
	conn   *grpc.ClientConn
	client protocol.NodeServiceClient
}

type peerState struct { // storing details related to peer state
	id   string
	host string
	port int

	conn *grpcConn

	lastSuccess time.Time
	suspect     bool
}

/*
here we are following a better way for peer is dead kinda thing
when the first request will be sent, if it responds then okay
if the 2nd request is send iand it doesnt respond back then it is updated to peer suspect state
and if the 3rd request is sent and it doesnt respond either then it returns peer dead state

this is a deterministic way to determine that the peer might be dead
*/

type nodeSession struct {
	nodeID   string
	nodePort int // current node addr and port
	nodeAddr string

	mu    sync.Mutex // introduced here so that we can lock when updating the struct
	peers map[string]*peerState
}

func (ns *nodeSession) getOrCreatePeer(
	ctx context.Context,
	peerID, host string,
	port int,
) (*peerState, error) {
	if !shouldDial(ns.nodeID, peerID) {
		return nil, fmt.Errorf("not allowed to dial peer: %s", peerID)
	}

	ns.mu.Lock()
	if ps, ok := ns.peers[peerID]; ok {
		ns.mu.Unlock()
		return ps, nil
	}
	ns.mu.Unlock()

	target := fmt.Sprintf("%s:%d", host, port)

	conn, err := grpc.NewClient(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	ps := &peerState{
		id:   peerID,
		host: host,
		port: port,
		conn: &grpcConn{
			conn:   conn,
			client: protocol.NewNodeServiceClient(conn),
		},
	}

	// perform handshake once
	if err := ns.handshake(ctx, ps); err != nil {
		_ = conn.Close()
		return nil, err
	}

	ns.mu.Lock()
	ns.peers[peerID] = ps
	ns.mu.Unlock()

	return ps, nil
}

// handshake is used to establish trust and verify that the peer is responsive before adding it to the session
func (ns *nodeSession) handshake(ctx context.Context, ps *peerState) error {
	opCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	_, err := ps.conn.client.Handshake(opCtx, &protocol.HandshakeRequest{
		NodeId: ns.nodeID,
		Addr:   ns.nodeAddr,
		Port:   int32(ns.nodePort),
	})
	return err
}

func (ns *nodeSession) Ping(
	ctx context.Context,
	peerID, host string,
	port int,
) PeerHealth {

	ps, err := ns.getOrCreatePeer(ctx, peerID, host, port)
	if err != nil {
		return PeerDead
	}

	opCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	_, err = ps.conn.client.Ping(opCtx, &protocol.PingRequest{
		From: ns.nodeID,
	})
	if err != nil {
		ns.markSuspect(peerID)
		return PeerSuspect
	}

	ns.markAlive(peerID)
	return PeerAlive
}

func (ns *nodeSession) markAlive(peerID string) {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	if ps, ok := ns.peers[peerID]; ok {
		ps.lastSuccess = time.Now()
		ps.suspect = false
	}
}

func (ns *nodeSession) markSuspect(peerID string) {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	if ps, ok := ns.peers[peerID]; ok {
		ps.suspect = true
	}
}

func shouldDial(self, peer string) bool {
	return self < peer
}
