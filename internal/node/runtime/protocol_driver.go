package runtime

import (
	"context"
	"log"
	"time"

	"github.com/vky5/faultlab/internal/fault"
	"github.com/vky5/faultlab/internal/node/protocol"
)

type ProtocolDriver struct {
	tickInterval time.Duration // after how many time we have to tick our logical clock (protocol.Tick()
	stopCh       chan struct{} // thsi syntax is for signal only channel (not for data) using int or any other means u are expecting data nd also will take memory
	eventCh      chan<- RuntimeEvent

	fault *fault.Engine
}

func NewProtocolDriver(
	tickInterval time.Duration,
	eventCh chan<- RuntimeEvent,
	fe *fault.Engine,
) *ProtocolDriver {
	return &ProtocolDriver{
		tickInterval: tickInterval,
		stopCh:       make(chan struct{}),
		eventCh:      eventCh,
		fault:        fe,
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
			if d.fault != nil && d.fault.IsCrashed() {
				continue
			}

			d.eventCh <- RuntimeEvent{
				Type: EventTick,
			}
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
