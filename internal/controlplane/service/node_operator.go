package service

import (
	"context"

	"github.com/vky5/faultlab/internal/protocol"
)

type NodeOperator interface {
	StopNode(ctx context.Context, host string, port int) error
	Ping(ctx context.Context, host string, port int) error
	ExecuteAction(ctx context.Context, host string, port int, req *protocol.ActionRequest) (*protocol.ActionResponse, error)
}

// we dont want that service depends on the nodeclient directly and want
// to keep the abstraction between them service don't see implementation of nodeclien that's why this was introduced

// The service layer depends on this interface instead of a concrete
// implementation (e.g., gRPC client) to enforce dependency inversion.
// This keeps orchestration logic decoupled from transport mechanisms,
// allowing different implementations (RPC, async queue, simulation, mocks)
// without modifying service logic.
//
// This interface represents a behavioral contract, not a transport detail.
