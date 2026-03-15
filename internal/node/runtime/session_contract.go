package runtime

import (
	"context"

	"github.com/vky5/faultlab/internal/protocol"
)

// PeerHealth represents the health status of a peer connection
type PeerHealth int

const (
	PeerAlive PeerHealth = iota
	PeerSuspect
	PeerDead
)

// PeerInfo represents peer topology information passed from runtime
type PeerInfo struct {
	ID   string
	Host string
	Port int
}

type CPSession interface {
	Establish(ctx context.Context) error
	Heartbeat(ctx context.Context) error
	FetchPeers(ctx context.Context) ([]*protocol.NodeInfo, error)
}

// NodeSession manages peer interactions and connection lifecycle
type NodeSession interface {
	OnPeersUpdated(peers []PeerInfo)
	Start(ctx context.Context)          // start internal loops
	GetPeerHealth(id string) PeerHealth // runtime reads state
}
