package runtime

import (
	"fmt"
	"os"
	"sync"

	"github.com/vky5/faultlab/internal/node"
	"google.golang.org/grpc"
)

type Runtime struct {
	config node.NodeConfig
	server *grpc.Server
	peersMu sync.RWMutex // needed so we don't update peers and ping the peer list at the same time
} 
// I get it now. Mutex is just a lock and we mentally think where we need and want to use it.
// it doesnt need to linked to a variable to which we need to read or write. this could be handelled by ourself

func New(cfg node.NodeConfig) Runtime {
	return Runtime{
		config: cfg,
	}
}

// controls wht happens in nodes during starting
func (r *Runtime) Start() {
	fmt.Printf("starting node %s on port %d\n", r.config.ID, r.config.Port)

	r.server = node.NewServer(r)
	go node.StartGRPCServer(r.config, r.server)

	go r.startPingLoop()
	go r.controlPlaneSyncLoop()

	select {}
}

// to stop the RPC server and node
func (r *Runtime) Stop() {
	if r.server == nil {
		return
	}

	r.server.GracefulStop()

	os.Exit(0) // exit the process after stopping the server 

	// ! we are exiting the process but defer will not run so be careful, create a separate func to handle cleanup
}
