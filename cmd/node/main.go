package main

import (
	"flag"
	"log"

	"github.com/vky5/faultlab/internal/node"
	noderuntime "github.com/vky5/faultlab/internal/node/runtime"
)

func main() {

	id := flag.String("id", "", "node id")
	port := flag.Int("port", 0, "port")
	peersFlag := flag.String("peers", "", "0")
	clusterID := flag.String("cluster-id", "default", "cluster id")
	host := flag.String("host", "localhost", "node advertised host/address")
	controlPlaneHost := flag.String("cp-host", "localhost", "control plane host")
	controlPlanePort := flag.Int("cp-port", 9000, "control plane gRPC port")

	flag.Parse()

	cfg, err := node.NewConfig(*id, *port, *peersFlag)
	if err != nil {
		log.Fatalf("invalid node config: %v", err)
	}
	cfg.ClusterID = *clusterID
	cfg.Host = *host
	cfg.ControlPlaneHost = *controlPlaneHost
	cfg.ControlPlanePort = *controlPlanePort

	runtime := noderuntime.New(cfg)

	runtime.Start()
}
