package runtime

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/vky5/faultlab/internal/node"
	proto "github.com/vky5/faultlab/internal/node/protocol"
	_ "github.com/vky5/faultlab/internal/node/protocol/baseline" // to register baseline
	"github.com/vky5/faultlab/internal/protocol"
	"google.golang.org/grpc"
)

type Runtime struct {
	config  node.NodeConfig
	server  *grpc.Server
	peersMu sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
	cp      CPSession
	ns      NodeSession

	proto  proto.ClusterProtocol
	driver *ProtocolDriver
}

func New(cfg node.NodeConfig, cp CPSession, ns NodeSession) Runtime {
	return Runtime{
		config: cfg,
		cp:     cp,
		ns:     ns,
	}
}

// controls wht happens in nodes during starting
func (r *Runtime) Start() {
	fmt.Printf("starting node %s on port %d\n", r.config.ID, r.config.Port)

	r.ctx, r.cancel = context.WithCancel(context.Background())

	r.server = node.NewServer(r) // implementing gRPC serverrs

	log.Printf("[runtime] Loading baseline protocol...")
	p, err := proto.Load("baseline")
	if err != nil {
		log.Fatalf("protocol load failed: %v", err)
	}
	r.proto = p
	log.Printf("[runtime] Protocol loaded successfully: %T", p)

	driver := NewProtocolDriver(
		1*time.Second, // time duration for each tick()
		func(ctx context.Context, env proto.Envelope) {
			r.ns.Send(ctx, env)
		},
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

	if err := r.proto.Start(r.config.ID); err != nil {
		log.Fatalf("failed to initialize the initial state of the protocol")
	}
	log.Printf("[runtime] Protocol started for node %s", r.config.ID)

	go r.driver.Run(r.ctx, p)

	// Start session's internal probe loop (sends actual pings, updates health state)
	go r.ns.Start(r.ctx)

	go r.runGRPCServer(r.ctx)
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

func (r *Runtime) HandleEnvelope(req *protocol.EnvelopeRequest) {

	env := proto.Envelope{
		From:     req.From,
		To:       req.To,
		Protocol: proto.ProtocolID(req.Protocol),
		Payload:  req.Payload,
	}

	out := r.proto.OnMessage(env)

	for _, e := range out {
		_ = r.ns.Send(r.ctx, e)
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
