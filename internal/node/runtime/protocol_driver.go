package runtime

import (
	"context"
	"log"
	"time"

	"github.com/vky5/faultlab/internal/node/exec"
	"github.com/vky5/faultlab/internal/node/protocol"
)

type ProtocolDriver struct {
	tickInterval time.Duration // after how many time we have to tick our logical clock (protocol.Tick()
	stopCh       chan struct{} // thsi syntax is for signal only channel (not for data) using int or any other means u are expecting data nd also will take memory
	eventCh      chan<- RuntimeEvent

	fault exec.FaultDecider
}

func NewProtocolDriver(
	tickInterval time.Duration,
	eventCh chan<- RuntimeEvent,
	faultDecider exec.FaultDecider,
) *ProtocolDriver {
	return &ProtocolDriver{
		tickInterval: tickInterval,
		stopCh:       make(chan struct{}),
		eventCh:      eventCh,
		fault:        faultDecider,
	}
}

// Create the intial state of the protocols
func (d *ProtocolDriver) Start(nodeID string, proto protocol.ClusterProtocol) {
	if err := proto.Start(nodeID); err != nil {
		log.Fatalf("protocol start failed: %v", err)
	}

}

func (d *ProtocolDriver) Run(ctx context.Context, proto protocol.ClusterProtocol) {
	ticker := time.NewTicker(d.tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tick := exec.TickDecision{Allow: true, Reason: "no-fault-decider"}
			if d.fault != nil {
				tick = d.fault.BeforeTick()
			}
			if !tick.Allow {
				log.Printf("protocol tick skipped: reason=%s", tick.Reason)
				continue
			}

			if tick.Delay > 0 {
				log.Printf("protocol tick delayed by %v", tick.Delay)
				select {
				case <-time.After(tick.Delay):
				case <-ctx.Done():
					return
				case <-d.stopCh:
					return
				}
			}

			d.eventCh <- RuntimeEvent{Type: EventTick}
		case <-d.stopCh:
			return
		}
	}
}

func (d *ProtocolDriver) Stop(proto protocol.ClusterProtocol) {
	close(d.stopCh)
	_ = proto.Stop()
}

//runtime loop → Tick() → protocol evolves → emits envelopes

/*
* Protocol emit messages via Tick() like what to do after certain time and
* then we process it in the runtime. We use session_contract to communicate to RPC,
* What I am thinking is creating separate proto files for communication based on the RPC


 */

// ? Making the protocol driver won the protocol seems better design to me
// ? but hotswapping will be difficult and the through runtime controlling the state of the protocol being used will also be difficult
// ? for example if I need to run 2 or 3 protocols parellely then
