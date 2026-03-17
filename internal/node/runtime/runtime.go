package runtime

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/vky5/faultlab/internal/node"
	proto "github.com/vky5/faultlab/internal/node/protocol"
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

func New(cfg node.NodeConfig, cp CPSession, ns NodeSession, driver *ProtocolDriver) Runtime {
	return Runtime{
		config: cfg,
		cp:     cp,
		ns:     ns,
		driver: driver,
	}
}

// controls wht happens in nodes during starting
func (r *Runtime) Start() {
	fmt.Printf("starting node %s on port %d\n", r.config.ID, r.config.Port)

	r.ctx, r.cancel = context.WithCancel(context.Background())

	r.server = node.NewServer(r)

	p, err := proto.Load("baseline") // TODO either make it as input or make it dynamic and the state is tied to x because under the hood x is BaseLineProtocol struct.
	if err != nil {
		log.Fatalf("protocol load failed: %v", err)
	}

	r.driver.Start(r.config.ID, p) // configuring default state of the protocol

	go r.driver.Run(p)

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
		log.Fatalf("failed to start the server: Retry again: %d ", err)
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

func (r *Runtime) handleOutBound(envs []proto.Envelope) {
	for _, e := range envs {
		r.ns.Send(r.ctx, e)
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
