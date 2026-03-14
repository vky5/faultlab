package runtime

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/vky5/faultlab/internal/node"
	"google.golang.org/grpc"
)

type cpsession interface {
	RegisterNodeWithControlPlane(ctx context.Context)
}

type Runtime struct {
	config  node.NodeConfig
	server  *grpc.Server
	peersMu sync.RWMutex    // needed so we don't update peers and ping the peer list at the same time
	ctx     context.Context // lifecycle context (think of it like main power supply for all node processes and other ctx as switches)
	cancel  context.CancelFunc
	cp      CPSession
}

// runtime <- server

func New(cfg node.NodeConfig, cp CPSession) Runtime {
	return Runtime{
		config: cfg,
		cp:     cp,
	}
}

// controls wht happens in nodes during starting
func (r *Runtime) Start() {
	fmt.Printf("starting node %s on port %d\n", r.config.ID, r.config.Port)

	r.ctx, r.cancel = context.WithCancel(context.Background())

	r.server = node.NewServer(r)

	go r.runGRPCServer(r.ctx)
	go r.startPingLoop(r.ctx)
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

/*
we are using service at controlplane because we are taking the decioion from cli like starting new node or stuff like that
but here the node is like and independent process that needs to be executed.
*/
