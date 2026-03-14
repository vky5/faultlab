package runtime

import (
	"context"

	"github.com/vky5/faultlab/internal/protocol"
)

type CPSession interface {
	Establish(ctx context.Context) error
	Heartbeat(ctx context.Context) error
	FetchPeers(ctx context.Context) ([]*protocol.NodeInfo, error)
}