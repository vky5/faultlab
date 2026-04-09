package runtime

import (
	proto "github.com/vky5/faultlab/internal/node/protocol"
	"github.com/vky5/faultlab/internal/protocol"
)

// runProtocolLoop is the single-threaded event reactor that processes all protocol events.
// It maintains deterministic ordering by sequentially handling EventTick, EventMessage, and EventAction.
// All mutations to protocol state occur within this goroutine, eliminating race conditions.
func (r *Runtime) runProtocolLoop() {
	r.logger.Printf("protocol reactor started")

	for {
		select {
		case <-r.ctx.Done():
			r.logger.Printf("protocol reactor stopping")
			return

		case ev := <-r.eventCh:
			switch ev.Type {

			case EventTick:
				out := r.proto.Tick()
				for _, e := range out {
					r.sendEnvelope(&e)
				}

			case EventMessage:
				out := r.proto.OnMessage(*ev.Msg)
				for _, e := range out {
					r.sendEnvelope(&e)
				}

			case EventAction:
				r.handleActionEvent(ev)

			case EventProtocolSwap:
				err := r.applyProtocolSwap(ev.ProtocolKey)
				if err == nil {
					// Report updated capabilities to Control Plane in background
					go func(key string) {
						if reportErr := r.reportCapabilitiesToControlPlane(r.ctx, key, 0); reportErr != nil {
							r.logger.Printf("failed to report capabilities: %v", reportErr)
						}
					}(ev.ProtocolKey)
				} else {
					r.logger.Printf("protocol swap failed: %v", err)
				}
				if ev.SwapErr != nil {
					ev.SwapErr <- err
				}
			}

		}
	}
}

// sendEnvelope handles both unicast and broadcast message delivery. Broadcast
// messages (To == "") are expanded to send individual messages to all known peers.
// Unicast messages are sent directly to the target peer.
// Each outbound message goes through the node session for actual network delivery.
func (r *Runtime) sendEnvelope(env *proto.Envelope) {
	// Broadcast: To == "" means send to all peers
	if env.To == "" {
		r.peersMu.RLock()
		peers := make([]string, 0, len(r.config.Peers))
		for _, p := range r.config.Peers {
			peers = append(peers, p.ID)
		}
		r.peersMu.RUnlock()

		r.logger.Printf("broadcast %s from %s to %d peers", env.Protocol, env.From, len(peers))
		for _, peerID := range peers {
			unicast := &proto.Envelope{
				From:     env.From,
				To:       peerID,
				Protocol: env.Protocol,
				Kind:     env.Kind,
				Payload:  env.Payload,
			}
			if err := r.ns.Send(r.ctx, *unicast); err != nil {
				r.logger.Printf("broadcast send to %s failed: %v", peerID, err)
			}
		}
		return
	}

	// Unicast: send to specific peer
	if err := r.ns.Send(r.ctx, *env); err != nil {
		r.logger.Printf("send to %s failed: %v", env.To, err)
	}
}

// HandleEnvelope receives incoming envelope messages from the node session
// and enqueues them as EventMessage into the protocol reactor.
// This ensures all protocol processing is single-threaded and deterministic.
func (r *Runtime) HandleEnvelope(req *protocol.EnvelopeRequest) {
	env := proto.Envelope{
		From:     req.From,
		To:       req.To,
		Protocol: proto.ProtocolID(req.Protocol),
		Payload:  req.Payload,
	}

	r.eventCh <- RuntimeEvent{
		Type: EventMessage,
		Msg:  &env,
	}
}
