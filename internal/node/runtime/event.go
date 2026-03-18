package runtime

import proto "github.com/vky5/faultlab/internal/node/protocol"

type RuntimeEventType int

const (
	EventTick RuntimeEventType = iota // Internal timer
	EventMessage // Network message received
)

/*
const (
	EventTick RuntimeEventType = 0
	EventMessage RuntimeEventType = 1
)
so when I type Type in RuntimeEvent it is actually 0 1 like this; and EventTick  and EventMessage is actually the const variables  that are used to refer to these numbered values
*/

type RuntimeEvent struct {
	Type RuntimeEventType  // types to represent event inside runtime
	Msg  *proto.Envelope
}
