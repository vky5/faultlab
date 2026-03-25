package runtime

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/vky5/faultlab/internal/fault"
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

	cp CPSession
	ns NodeSession

	proto  proto.ClusterProtocol
	driver *ProtocolDriver

	eventCh chan RuntimeEvent

	fault *fault.Engine // inject the faults in the same node's runtime
	logger *Logger
}

func New(cfg node.NodeConfig, cp CPSession, ns NodeSession, runtimeConfig config.NodeRuntimeConfig) Runtime {
	return Runtime{
		config:        cfg,
		runtimeConfig: runtimeConfig,
		cp:            cp,
		ns:            ns,
		logger:        NewLogger(cfg.ID, "runtime", cp),
	}
}

// controls wht happens in nodes during starting
func (r *Runtime) Start(fe *fault.Engine) {
	r.logger.Printf("starting node %s on port %d", r.config.ID, r.config.Port)

	r.ctx, r.cancel = context.WithCancel(context.Background())
	r.server = node.NewServer(r)              // implementing gRPC serverrs
	r.eventCh = make(chan RuntimeEvent, 1024) // implementing channel to recieve events // TODO check for the backpressure

	r.logger.Printf("Loading baseline protocol...")
	p, err := proto.Load("baseline")
	if err != nil {
		log.Fatalf("protocol load failed: %v", err)
	}
	r.proto = p
	r.logger.Printf("Protocol loaded successfully: %T", p)

	// initializing fault injection engine
	r.fault = fe

	driver := NewProtocolDriver(
		1*time.Second, // time duration for each tick()
		r.eventCh,
		// func(ctx context.Context, env proto.Envelope) {
		// 	r.ns.Send(ctx, env)
		// },
		fe,
	)

	r.driver = driver

	// Initialize protocol with initial peer list
	peerIDs := make([]string, 0, len(r.config.Peers))
	for _, p := range r.config.Peers {
		peerIDs = append(peerIDs, p.ID)
	}
	if baseline, ok := p.(interface{ SetPeers([]string) }); ok {
		baseline.SetPeers(peerIDs)
		r.logger.Printf("Initial peers set for protocol: %v", peerIDs)
	}

	if pWithDiscovery, ok := p.(proto.ClusterProtocolWithDiscovery); ok {
		pWithDiscovery.SetPeerDiscoveryCallback(r)
		r.logger.Printf("Registered peer discovery callback")
	}

	if err := r.proto.Start(r.config.ID); err != nil {
		log.Fatalf("failed to initialize the initial state of the protocol")
	}
	r.logger.Printf("Protocol started for node %s", r.config.ID)

	go r.driver.Run(r.ctx, p)
	go r.runProtocolLoop()
	go r.ns.Start(r.ctx)      // Start node session's internal probe loop (sends actual pings, updates health state)
	go r.runGRPCServer(r.ctx) // Start gRPC server FIRST and wait for it to be ready

	time.Sleep(500 * time.Millisecond) // Wait briefly for gRPC server to start listening before registering

	go r.controlPlaneSyncLoop(r.ctx)

	// * Uncomment to simulate the crash and recovery of the node
	// go func() {
	// 	time.Sleep(10 * time.Second)
	// 	log.Println("***** CRASHING NODE *****")
	// 	r.fault.Crash()
	// 	time.Sleep(50 * time.Second)
	// 	log.Printf("**** Starting Recovery ****")
	// 	r.fault.Recover()
	// }()

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

// IsCrashed reports whether this node is currently fault-injected as crashed.
func (r *Runtime) IsCrashed() bool {
	return r.fault != nil && r.fault.IsCrashed()
}

/*
If ctx cancelled, return
if there is event, there can be two types of event

	EventTick -> Triggered by Tick
	EventMessage -> Triggered when message is sent

(this actually works by recieving messages from ProtocolDriver)
*/
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
			}
		}
	}
}

// sendEnvelope handles unicast and broadcast delivery
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
			r.logger.Printf("TRACE:SEND:%s:%s", env.From, peerID)
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
	r.logger.Printf("TRACE:SEND:%s:%s", env.From, env.To)
	if err := r.ns.Send(r.ctx, *env); err != nil {
		r.logger.Printf("send to %s failed: %v", env.To, err)
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
	r.logger.Printf("dynamically discovered peer %s at %s:%d", peerID, peerHost, peerPort)

	if r.ctx == nil {
		return
	}
	if peerID == "" || peerPort <= 0 {
		r.logger.Printf("ignoring invalid discovered peer id=%q host=%q port=%d", peerID, peerHost, peerPort)
		return
	}
	if peerHost == "" {
		peerHost = "localhost"
	}

	r.peersMu.RLock()
	base := make([]*protocol.NodeInfo, 0, len(r.config.Peers))
	for _, p := range r.config.Peers {
		base = append(base, &protocol.NodeInfo{Id: p.ID, Address: p.Host, Port: uint32(p.Port)})
	}
	r.peersMu.RUnlock()

	r.applyPeersTopology(base, &protocol.NodeInfo{Id: peerID, Address: peerHost, Port: uint32(peerPort)})
}

// setter for the parameter of fault
func (r *Runtime) SetFaultParams(params *protocol.FaultRequest) error {
	if params == nil {
		return fmt.Errorf("fault params request is nil")
	}

	if r.fault == nil {
		return fmt.Errorf("fault engine is not initialized")
	}

	if params.GetNodeId() != "" && params.GetNodeId() != r.config.ID {
		return fmt.Errorf("fault request node mismatch: got=%s want=%s", params.GetNodeId(), r.config.ID)
	}

	dropRate := params.GetDropRate()
	if dropRate < 0 || dropRate > 1 {
		return fmt.Errorf("drop_rate must be in range [0,1], got=%f", dropRate)
	}

	delayMs := params.GetDelayMs()
	if delayMs < 0 {
		return fmt.Errorf("delay_ms must be >= 0, got=%d", delayMs)
	}

	if params.GetCrashed() {
		r.fault.Crash()
	} else {
		r.fault.Recover()
	}

	r.fault.SetDropRate(dropRate)
	r.fault.SetDelay(int(delayMs))

	// Treat request.partition as the desired partition set.
	desired := make(map[string]struct{}, len(params.GetPartition()))
	for _, id := range params.GetPartition() {
		if id == "" || id == r.config.ID {
			continue
		}
		desired[id] = struct{}{}
	}

	r.peersMu.RLock()
	knownPeers := make([]string, 0, len(r.config.Peers))
	for _, p := range r.config.Peers {
		knownPeers = append(knownPeers, p.ID)
	}
	r.peersMu.RUnlock()

	for _, id := range knownPeers {
		if _, ok := desired[id]; ok {
			r.fault.Partition(id)
		} else {
			r.fault.Heal(id)
		}
	}

	// Also allow partitioning peers that are not currently in runtime config yet.
	for id := range desired {
		r.fault.Partition(id)
	}

	r.logger.Printf("fault params applied: crashed=%v drop_rate=%.3f delay_ms=%d partition=%v", params.GetCrashed(), dropRate, delayMs, params.GetPartition())

	return nil
}
