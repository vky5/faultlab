package main

import (
	"flag"

	"github.com/vky5/faultlab/internal/node"
)

func main() {

	id := flag.String("id", "", "node id")
	port := flag.Int("port", 0, "port")

	flag.Parse()

	cfg := node.NodeConfig{
		ID:   *id,
		Port: *port,
	}

	runtime := node.New(cfg)

	runtime.Start()
}
