package node

import (
	"fmt"

	"google.golang.org/grpc"
)

type Runtime struct {
	config NodeConfig
	server *grpc.Server
}

func New(cfg NodeConfig) Runtime {
	return Runtime{
		config: cfg,
	}
}

func (r *Runtime) Start() error {
	fmt.Printf("starting node %s on port %d\n", r.config.ID, r.config.Port)
	r.server = NewServer(r.config)
	StartGRPCServer(r.config, r.server)
	return nil
}

func (r *Runtime) Stop() {
	if r.server == nil {
		return
	}

	r.server.GracefulStop()
}
