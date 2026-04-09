package main

import (
	"fmt"
	"github.com/vky5/faultlab/internal/node/protocol/gossip"
)

type KVPut interface {
	Put(key, value string)
}

func main() {
	g := gossip.NewGossipProtocol()
	var i any = g
	_, ok := i.(KVPut)
	fmt.Printf("Implements KVPut: %v\n", ok)
}
