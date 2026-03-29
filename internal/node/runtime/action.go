package runtime

import (
	"context"

	"google.golang.org/protobuf/proto"

	"github.com/vky5/faultlab/internal/protocol"
)

// handleActionEvent processes action requests in the deterministic reactor loop.
// It decodes the action type and dispatches to the appropriate handler (KV_PUT, KV_GET),
// sending the response back through the event's response channel.
func (r *Runtime) handleActionEvent(ev RuntimeEvent) {
	req := ev.Act
	respCh := ev.Resp

	// safety (don't trust inputs)
	if req == nil {
		respCh <- fail("nil action request")
		return
	}

	switch req.Action {

	case protocol.ActionType_KV_PUT:
		var p protocol.KVPutRequest

		if err := proto.Unmarshal(req.Payload, &p); err != nil {
			respCh <- fail("invalid KVPut payload")
			return
		}

		if err := PutAction(r.proto, p.Key, p.Value); err != nil {
			respCh <- fail(err.Error())
			return
		}

		respCh <- success(nil)

	case protocol.ActionType_KV_GET:
		var p protocol.KVGetRequest

		if err := proto.Unmarshal(req.Payload, &p); err != nil {
			respCh <- fail("invalid KVGet payload")
			return
		}

		val, found, err := GetAction(r.proto, p.Key)
		if err != nil {
			respCh <- fail(err.Error())
			return
		}

		if !found {
			respCh <- fail("key not found")
			return
		}

		bytes, err := proto.Marshal(&protocol.KVGetResponse{Value: val})
		if err != nil {
			respCh <- fail("serialization failed")
			return
		}

		respCh <- success(bytes)

	default:
		respCh <- fail("unknown action")
	}
}

// ExecuteAction enqueues an action request into the protocol reactor loop
// and waits for the response with context deadline support.
func (r *Runtime) ExecuteAction(
	ctx context.Context,
	req *protocol.ActionRequest,
) (*protocol.ActionResponse, error) {

	if req == nil {
		return &protocol.ActionResponse{
			Success: false,
			Message: "nil action request",
		}, nil
	}

	respCh := make(chan *protocol.ActionResponse, 1)

	r.eventCh <- RuntimeEvent{
		Type: EventAction,
		Act:  req,
		Resp: respCh,
	}

	select {
	case resp := <-respCh:
		return resp, nil

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// success builds a successful ActionResponse with optional payload.
func success(payload []byte) *protocol.ActionResponse {
	return &protocol.ActionResponse{
		Success: true,
		Payload: payload,
	}
}

// fail builds a failure ActionResponse with an error message.
func fail(msg string) *protocol.ActionResponse {
	return &protocol.ActionResponse{
		Success: false,
		Message: msg,
	}
}
