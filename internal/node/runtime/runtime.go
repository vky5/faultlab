package runtime

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/vky5/faultlab/internal/node"
	"github.com/vky5/faultlab/internal/node/config"
	proto "github.com/vky5/faultlab/internal/node/protocol"
	_ "github.com/vky5/faultlab/internal/node/protocol/baseline" // to register baseline
	"github.com/vky5/faultlab/internal/protocol"
	"google.golang.org/grpc"
)

type Runtime struct {
	config        node.NodeConfig
	runtimeConfig config.NodeRuntimeConfig
	server        *grpc.Server
	peersMu       sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	cp            CPSession
	ns            NodeSession

	proto  proto.ClusterProtocol
	driver *ProtocolDriver

	eventCh chan RuntimeEvent
}

func New(cfg node.NodeConfig, cp CPSession, ns NodeSession, runtimeConfig config.NodeRuntimeConfig) Runtime {
	return Runtime{
		config:        cfg,
		runtimeConfig: runtimeConfig,
		cp:            cp,
		ns:            ns,
	}
}

// controls wht happens in nodes during starting
func (r *Runtime) Start() {
	fmt.Printf("starting node %s on port %d\n", r.config.ID, r.config.Port)

	r.ctx, r.cancel = context.WithCancel(context.Background())
	r.server = node.NewServer(r)              // implementing gRPC serverrs
	r.eventCh = make(chan RuntimeEvent, 1024) // implementing channel to recieve events // TODO check for the backpressure

	log.Printf("[runtime] Loading baseline protocol...")
	p, err := proto.Load("baseline")
	if err != nil {
		log.Fatalf("protocol load failed: %v", err)
	}
	r.proto = p
	log.Printf("[runtime] Protocol loaded successfully: %T", p)

	driver := NewProtocolDriver(
		1*time.Second, // time duration for each tick()
		r.eventCh,
		// func(ctx context.Context, env proto.Envelope) {
		// 	r.ns.Send(ctx, env)
		// },
	)

	r.driver = driver

	// Initialize protocol with initial peer list
	peerIDs := make([]string, 0, len(r.config.Peers))
	for _, p := range r.config.Peers {
		peerIDs = append(peerIDs, p.ID)
	}
	if baseline, ok := p.(interface{ SetPeers([]string) }); ok {
		baseline.SetPeers(peerIDs)
		log.Printf("[runtime] Initial peers set for protocol: %v", peerIDs)
	}

	if pWithDiscovery, ok := p.(proto.ClusterProtocolWithDiscovery); ok {
		pWithDiscovery.SetPeerDiscoveryCallback(r)
		log.Printf("[runtime] Registered peer discovery callback")
	}

	if err := r.proto.Start(r.config.ID); err != nil {
		log.Fatalf("failed to initialize the initial state of the protocol")
	}
	log.Printf("[runtime] Protocol started for node %s", r.config.ID)

	go r.driver.Run(r.ctx, p)

	go r.runProtocolLoop()

	// Start session's internal probe loop (sends actual pings, updates health state)
	go r.ns.Start(r.ctx)

	// Start gRPC server FIRST and wait for it to be ready
	go r.runGRPCServer(r.ctx)

	// Wait briefly for gRPC server to start listening before registering
	time.Sleep(500 * time.Millisecond)

	go r.controlPlaneSyncLoop(r.ctx)

	<-r.ctx.Done()
}

func (r *Runtime) runGRPCServer(ctx context.Context) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", r.config.Port))
	if err != nil {
		// runtime decides what to do
		return
	}

	go func() {
		<-ctx.Done()
		r.server.GracefulStop()
	}()

	if err := r.server.Serve(lis); err != nil {
		log.Fatalf("failed to start the server: Retry again: %v", err)
	}
}

// to stop the RPC server and node
func (r *Runtime) Stop() {
	if r.cancel != nil {
		r.cancel()
	}

	if r.server != nil {
		r.server.GracefulStop() // closing grpc server
	}
}

/*
If ctx cancelled, return
if there is event, there can be two types of event

	EventTick -> Triggered by Tick
	EventMessage -> Triggered when message is sent

(this actually works by recieving messages from ProtocolDriver)
*/
func (r *Runtime) runProtocolLoop() {
	log.Println("[runtime] protocol reactor started")

	for {
		select {
		case <-r.ctx.Done():
			log.Println("[runtime] protocol reactor stopping")
			return

		case ev := <-r.eventCh:
			switch ev.Type {

			case EventTick:
				out := r.proto.Tick()
				for _, e := range out {
					if err := r.ns.Send(r.ctx, e); err != nil {
						log.Println("SEND ERROR:", err)
					}
				}

			case EventMessage:
				out := r.proto.OnMessage(*ev.Msg)
				for _, e := range out {
					if err := r.ns.Send(r.ctx, e); err != nil {
						log.Println("SEND ERROR:", err)
					}
				}
			}
		}
	}
}

/*
from a separate node first
- session gets a message
- Passes it to HandleEnvelope in runtime
- It passes it to r.eventCh of type RuntimeEvent
- EventCh has two types of message either tick or These Messages (making it single looped deterministic)
*/
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

/*
we are using service at controlplane because we are taking the decioion from cli like starting new node or stuff like that
but here the node is like and independent process that needs to be executed.
*/

/*
Protocol.Tick()
   → returns []Envelope
Runtime
   → calls NodeSession.Send(env)
NodeSession
   → gRPC send
Peer RPC server
   → Runtime.HandleEnvelope(env)
Runtime
   → proto.OnMessage(env)
   → maybe emits response envelopes
   → NodeSession.Send again

basically only one session at a time and that's it
*/

// OnPeerDiscovered receives dynamically discovered peers from the protocol.
func (r *Runtime) OnPeerDiscovered(peerID, peerHost string, peerPort int) {
	log.Printf("[runtime] dynamically discovered peer %s at %s:%d", peerID, peerHost, peerPort)
	r.ns.RegisterPeer(peerID, peerHost, peerPort)
}

