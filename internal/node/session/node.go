package session

import (
	"context"
	"fmt"
	"sync"
	"time"

	proto "github.com/vky5/faultlab/internal/node/protocol"
	noderuntime "github.com/vky5/faultlab/internal/node/runtime"
	"github.com/vky5/faultlab/internal/protocol"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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
	failCount   int
	health      noderuntime.PeerHealth
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

	mu    sync.RWMutex // introduced here so that we can lock when updating the struct
	peers map[string]*peerState

	probeInterval time.Duration
}

// sending message to other node (doesnt check for should connect)
func (ns *nodeSession) Send(ctx context.Context, env proto.Envelope) error {
	ns.mu.Lock()
	ps, ok := ns.peers[env.To]
	ns.mu.Unlock()

	if !ok {
		return fmt.Errorf("peer not found: %s", env.To)
	}

	// lazy connect
	if ps.conn == nil {
		c, err := ns.dial(ctx, ps.host, ps.port)
		if err != nil {
			return err
		}

		ps.conn = c

		if err := ns.handshake(ctx, ps); err != nil {
			return err
		}
	}

	opCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	// encode host and port into From to allow in-band discovery
	fromStr := fmt.Sprintf("%s@%s:%d", env.From, ns.nodeAddr, ns.nodePort)

	_, err := ps.conn.client.SendEnvelope(opCtx, &protocol.EnvelopeRequest{
		From:     fromStr,
		To:       env.To,
		Protocol: string(env.Protocol),
		Payload:  env.Payload,
	})

	if err != nil {
		ns.updateFailure(ps)
		return err
	}

	ns.updateSuccess(ps)
	return nil
}

// starting the initial connection with all nodes in a cluster 
func (ns *nodeSession) Start(ctx context.Context) {
	fmt.Printf("[node:%s] starting probe loop (interval=%v)\n", ns.nodeID, ns.probeInterval)
	ticker := time.NewTicker(ns.probeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Printf("[node:%s] stopping probe loop\n", ns.nodeID)
			ns.closeAllConnections()
			return
		case <-ticker.C:
			ns.probeOnce(ctx) // send ping to all peers and updatre peer state accordingly
		}
	}
}

// sends the request to all peers at least once. It does check the shoulddial() so there are two times it is checked once in probepeers other in probe ocne 
func (ns *nodeSession) probeOnce(ctx context.Context) {
	ns.mu.Lock()
	peers := make([]*peerState, 0, len(ns.peers))
	for _, ps := range ns.peers {
		peers = append(peers, ps)
	}
	ns.mu.Unlock()

	fmt.Printf("[node:%s] probing %d peers...\n", ns.nodeID, len(peers))
	for _, ps := range peers {
		// deterministic dialing: only owner initiates probes
		if !shouldDial(ns.nodeID, ps.id) {
			continue
		}
		ns.probePeer(ctx, ps)
	}
}

// sends the connection request to the peer in the peer state (it does check should dial) and update the peer state with updateSuccess()/ updateFailure()  and connection info 
func (ns *nodeSession) probePeer(ctx context.Context, ps *peerState) {
	// deterministic dialing: only owner initiates probes
	if !shouldDial(ns.nodeID, ps.id) {
		return
	}

	// lazy connect
	if ps.conn == nil {
		c, err := ns.dial(ctx, ps.host, ps.port)
		if err != nil {
			fmt.Printf("[node:%s] probe %s: dial failed: %v\n", ns.nodeID, ps.id, err)
			ns.updateFailure(ps)
			return
		}
		ps.conn = c
		if err := ns.handshake(ctx, ps); err != nil {
			fmt.Printf("[node:%s] probe %s: handshake failed: %v\n", ns.nodeID, ps.id, err)
		}
	}

	opCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	_, err := ps.conn.client.Ping(opCtx, &protocol.PingRequest{From: ns.nodeID})
	if err != nil {
		fmt.Printf("[node:%s] probe %s (%s:%d): ping failed: %v\n", ns.nodeID, ps.id, ps.host, ps.port, err)
		ns.updateFailure(ps)
		return
	}

	fmt.Printf("[node:%s] probe %s (%s:%d): alive\n", ns.nodeID, ps.id, ps.host, ps.port)
	ns.updateSuccess(ps)
}

// Dial to any node if the host and port is known. This also stores the connection of that node in the peers 
func (ns *nodeSession) dial(ctx context.Context, host string, port int) (*grpcConn, error) {
	target := fmt.Sprintf("%s:%d", host, port)

	conn, err := grpc.NewClient(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	return &grpcConn{
		conn:   conn,
		client: protocol.NewNodeServiceClient(conn),
	}, nil
}

// loops through all active connection and close it all
func (ns *nodeSession) closeAllConnections() {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	for _, ps := range ns.peers {
		if ps.conn != nil && ps.conn.conn != nil {
			_ = ps.conn.conn.Close()
		}
	}
}

// updating success message for a peer that info stored in peer state
func (ns *nodeSession) updateSuccess(ps *peerState) {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	ps.lastSuccess = time.Now()
	ps.failCount = 0
	ps.health = noderuntime.PeerAlive
}

func (ns *nodeSession) updateFailure(ps *peerState) {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	ps.failCount++
	if ps.failCount == 1 {
		ps.health = noderuntime.PeerSuspect
	} else if ps.failCount >= 2 {
		ps.health = noderuntime.PeerDead
	}
}

// storing the peer info and based on the shouldDIal check, storing the conn from lower -> higher node and no connection if not that case
func (ns *nodeSession) getOrCreatePeer(
	ctx context.Context,
	peerID, host string,
	port int,
) (*peerState, error) {

	ns.mu.Lock()
	ps, ok := ns.peers[peerID]
	if !ok {
		ps = &peerState{
			id:     peerID,
			host:   host,
			port:   port,
			health: noderuntime.PeerSuspect,
		}
		ns.peers[peerID] = ps
	}
	ns.mu.Unlock()

	// If not dial owner → just return metadata
	if !shouldDial(ns.nodeID, peerID) {
		return ps, nil
	}

	// Dial owner ensures connection
	if ps.conn == nil {
		c, err := ns.dial(ctx, ps.host, ps.port)
		if err != nil {
			return nil, err
		}

		ps.conn = c

		if err := ns.handshake(ctx, ps); err != nil {
			_ = c.conn.Close()
			ps.conn = nil
			return nil, err
		}
	}

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

// OnPeersUpdated updates the peer topology based on changes from runtime.
// Runtime passes the authoritative peer list, session decides connection lifecycle.
// This inverts control: runtime owns topology, session owns peer interactions.
/*
stale connections closed
new peers prepared
session internal state stays consistent
runtime stops micromanaging networking
*/
func (ns *nodeSession) OnPeersUpdated(peers []noderuntime.PeerInfo) {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	// Build new topology map
	newSet := make(map[string]noderuntime.PeerInfo, len(peers))
	for _, p := range peers {
		newSet[p.ID] = p
	}

	// 1 Remove peers not present anymore
	for id, ps := range ns.peers {
		if _, ok := newSet[id]; !ok {
			if ps.conn != nil && ps.conn.conn != nil {
				_ = ps.conn.conn.Close()
			}
			delete(ns.peers, id)
		}
	}

	// 2️ Add new peers (lazy state only)
	for id, info := range newSet {
		if _, ok := ns.peers[id]; !ok {
			ns.peers[id] = &peerState{
				id:     info.ID,
				host:   info.Host,
				port:   info.Port,
				health: noderuntime.PeerSuspect,
			}
		}
	}

	// 3️ Update metadata if peer changed address
	for id, info := range newSet {
		if ps, ok := ns.peers[id]; ok {
			if ps.host != info.Host || ps.port != info.Port {
				// peer endpoint changed → reset connection
				if ps.conn != nil && ps.conn.conn != nil {
					_ = ps.conn.conn.Close()
				}
				ps.host = info.Host
				ps.port = info.Port
				ps.conn = nil
			}
		}
	}
}

// Read operation to check the health of the peer through their ID
func (ns *nodeSession) GetPeerHealth(id string) noderuntime.PeerHealth {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	if ps, ok := ns.peers[id]; ok {
		return ps.health
	}
	return noderuntime.PeerDead // unknown peer treated as dead
}

// RegisterPeer dynamically adds a newly discovered peer to the session
func (ns *nodeSession) RegisterPeer(peerID, host string, port int) {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	if ps, ok := ns.peers[peerID]; !ok {
		ns.peers[peerID] = &peerState{
			id:     peerID,
			host:   host,
			port:   port,
			health: noderuntime.PeerAlive, // Just discovered, assume alive
		}
	} else {
		// Update connection details if they changed
		if ps.host != host || ps.port != port {
			if ps.conn != nil && ps.conn.conn != nil {
				_ = ps.conn.conn.Close()
			}
			ps.host = host
			ps.port = port
			ps.conn = nil
		}
	}
}

func shouldDial(self, peer string) bool {
	return self < peer
}

// NewNodeSession creates a new node session for peer interactions
func NewNodeSession(nodeAddr string, nodeID string, nodePort int) noderuntime.NodeSession {
	return &nodeSession{
		nodeID:        nodeID,
		nodePort:      nodePort,
		nodeAddr:      nodeAddr,
		peers:         make(map[string]*peerState),
		probeInterval: 2 * time.Second,
	}
}
