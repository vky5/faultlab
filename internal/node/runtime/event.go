package runtime

import (
	proto "github.com/vky5/faultlab/internal/node/protocol"
	"github.com/vky5/faultlab/internal/protocol"
)

type RuntimeEventType int

const (
	EventTick    RuntimeEventType = iota // Internal timer
	EventMessage                         // Network message received
	EventAction                          // Action from the control plane
	EventProtocolSwap                     // Protocol swap request
)

/*
const (
	EventTick RuntimeEventType = 0
	EventMessage RuntimeEventType = 1
	EventAction RuntimeEventType = 2
)
so when I type Type in RuntimeEvent it is actually 0 1 like this; and EventTick  and EventMessage is actually the const variables  that are used to refer to these numbered values
*/

type RuntimeEvent struct {
	Type RuntimeEventType // types to represent event inside runtime
	Msg  *proto.Envelope  // for network message events

	// for control plane actions
	Act  *protocol.ActionRequest
	Resp chan *protocol.ActionResponse

	// for protocol swap requests
	ProtocolKey string
	SwapErr     chan error
}

/*
For each contorlplane event we are sending the resp channel as wel
on which we will get the response from the runtime

But what will happen if multiple request hits the server at the same time --- those will be stored in the eventCh and will be handled one at a time... 
*/