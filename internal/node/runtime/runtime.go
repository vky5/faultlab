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
	_ "github.com/vky5/faultlab/internal/node/protocol/gossip"   // to register gossip
	"google.golang.org/grpc"
)

// Runtime represents a node's runtime container that manages protocol execution,
// RPC communication, fault injection, and the deterministic event loop.
// It owns the lifecycle of the protocol reactor and coordinates all node operations.
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

	fault  *fault.Engine // inject the faults in the same node's runtime
	logger *Logger
}

// New creates a new Runtime instance with the given configuration and sessions.
// The runtime is not yet started; call Start() to begin operations.
func New(cfg node.NodeConfig, cp CPSession, ns NodeSession, runtimeConfig config.NodeRuntimeConfig) *Runtime {
	r := &Runtime{
		config:        cfg,
		runtimeConfig: runtimeConfig,
		cp:            cp,
		ns:            ns,
		logger:        NewLogger(cfg.ID, "runtime", cp),
	}
	ns.SetLogger(r.logger)
	return r
}

// Start initializes and runs the node runtime. It:
// - Loads the gossip protocol and initializes it with the peer list
// - Starts the protocol event loop (reactor) for deterministic processing
// - Starts the node session for peer communication
// - Starts the gRPC server for RPC endpoints
// - Starts the controlplane sync loop for topology changes
// - Runs until the context is cancelled
func (r *Runtime) Start(fe *fault.Engine) {
	r.logger.Printf("starting node %s on port %d", r.config.ID, r.config.Port)

	r.ctx, r.cancel = context.WithCancel(context.Background())
	r.server = node.NewServer(r)              // implementing gRPC serverrs
	r.eventCh = make(chan RuntimeEvent, 1024) // implementing channel to recieve events // TODO check for the backpressure

	// initializing fault injection engine
	r.fault = fe

	go r.runGRPCServer(r.ctx) // Start gRPC server first so controlplane verification ping can succeed

	time.Sleep(500 * time.Millisecond) // Wait briefly for gRPC server to start listening before registering

	go r.runProtocolLoop()

	assignedProtocol, err := r.registerNodeWithControlPlane(r.ctx)
	if err != nil {
		log.Fatalf("registration failed: %v", err)
	}

	if err := r.getPeersFromControlplane(r.ctx); err != nil {
		r.logger.Printf("initial peer sync failed: %v", err)
	}

	r.logger.Printf("Loading protocol assigned by controlplane via swap path: %s", assignedProtocol)
	if err := r.ProtocolSwap(r.ctx, assignedProtocol); err != nil {
		log.Fatalf("protocol initialization failed for %s: %v", assignedProtocol, err)
	}

	driver := NewProtocolDriver(
		1*time.Second, // time duration for each tick()
		r.eventCh,
		// func(ctx context.Context, env proto.Envelope) {
		// 	r.ns.Send(ctx, env)
		// },
		r,
	)

	r.driver = driver

	r.logger.Printf("Protocol started for node %s", r.config.ID)

	// PutAction(r.proto, "b", "node2_value")
	// PutAction(r.proto, "c", "node3_valufasde_2")
	// PutAction(r.proto, "d", "node4_fasdfasd")
	// PutAction(r.proto, "e", "node5_value")
	// PutAction(r.proto, "a", "node1_value")

	go r.driver.Run(r.ctx, r.proto)
	go r.ns.Start(r.ctx) // Start node session's internal probe loop (sends actual pings, updates health state)

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

// runGRPCServer starts the gRPC server on the configured port.
// It listens for incoming RPC connections until the context is cancelled.
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

// Stop gracefully shutdowns the runtime by cancelling the context,
// which triggers cleanup of all running goroutines and gRPC server.
func (r *Runtime) Stop() {
	if r.cancel != nil {
		r.cancel()
	}

	if r.server != nil {
		r.server.GracefulStop() // closing grpc server
	}
}
