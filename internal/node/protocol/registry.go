package protocol

import (
	"fmt"
	"sync"
)

type Factory func() ClusterProtocol // Factory returns ClusterProtocol

var (
	mu        sync.RWMutex
	factories = make(map[string]Factory) // this actually stores the string_name of the factory mapped to -> object of the struct that has controlplane impleemntation
)

// Registry is called by protocol implementation
func Register(name string, f Factory) {
	mu.Lock()
	defer mu.Unlock()

	if name == "" {
		panic("protocol: empty name")
	}

	if f == nil {
		panic("protocol: nil factory")
	}

	if _, exists := factories[name]; exists {
		panic(fmt.Sprintf("protocol: already registered: %s", name))
	}

	factories[name] = f
}

func Load(name string) (ClusterProtocol, error) {
	mu.RLock()
	f, ok := factories[name]
	mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("protocol: not found: %s", name)
	}

	return f(), nil

	/*
		? Everytime you do x, _:= Load("something")
		? y,_:= Load("something")

		? two separate loads struct of something protocol will be created and those two will be completely independent and their struct will be also different from each other
	*/
}

// list returns registered protocol names
func List() []string {
	mu.RLock()
	defer mu.RUnlock()

	out := make([]string, 0, len(factories))
	for k := range factories {
		out = append(out, k)
	}

	return out
}
