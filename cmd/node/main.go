package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/vky5/faultlab/internal/node"
	"github.com/vky5/faultlab/internal/node/config"
	noderuntime "github.com/vky5/faultlab/internal/node/runtime"
	"github.com/vky5/faultlab/internal/node/session"
)

func main() {
	configFile := flag.String("config", "", "path to node runtime config file (INI format)")

	id := flag.String("id", "", "node id")
	port := flag.Int("port", 0, "port")
	peersFlag := flag.String("peers", "", "comma-separated peer list")
	clusterID := flag.String("cluster-id", "default", "cluster id")
	host := flag.String("host", "localhost", "node advertised host/address")
	controlPlaneHost := flag.String("cp-host", "", "control plane host (overrides config file)")
	controlPlanePort := flag.Int("cp-port", 0, "control plane gRPC port (overrides config file)")

	flag.Parse()

	// Load runtime config from file if provided
	var runtimeCfg config.NodeRuntimeConfig
	var err error

	if *configFile != "" {
		runtimeCfg, err = config.LoadNodeRuntimeConfig(*configFile)
		if err != nil {
			log.Fatalf("failed to load runtime config: %v", err)
		}
		fmt.Printf("loaded runtime config from: %s\n", *configFile)
	} else {
		runtimeCfg = config.DefaultNodeRuntimeConfig()
		fmt.Println("using default runtime config")
	}

	// CLI flags override config file for controlplane connection
	if *controlPlaneHost != "" {
		runtimeCfg.ControlPlane.Host = *controlPlaneHost
	}
	if *controlPlanePort != 0 {
		runtimeCfg.ControlPlane.Port = *controlPlanePort
	}

	cfg, err := node.NewConfig(*id, *port, *peersFlag)
	if err != nil {
		log.Fatalf("invalid node config: %v", err)
	}
	cfg.ClusterID = *clusterID
	cfg.Host = *host
	cfg.ControlPlaneHost = runtimeCfg.ControlPlane.Host
	cfg.ControlPlanePort = runtimeCfg.ControlPlane.Port

	cpSession := session.NewControlplaneSession(
		cfg.ID,
		cfg.ClusterID,
		cfg.ControlPlaneHost,
		cfg.ControlPlanePort,
		cfg.Host,
		cfg.Port,
	)

	nodeSession := session.NewNodeSession(
		cfg.Host,
		cfg.ID,
		cfg.Port,
	)

	runtime := noderuntime.New(cfg, cpSession, nodeSession, runtimeCfg)

	runtime.Start()
}
