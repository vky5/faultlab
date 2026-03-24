package cluster

import "time"

// FaultState holds fault-injection parameters for one node.
type FaultState struct {
	Crashed   bool     `json:"crashed"`
	DropRate  float64  `json:"dropRate"`
	DelayMs   int      `json:"delayMs"`
	Partition []string `json:"partition,omitempty"`
}

func DefaultFaultState() FaultState {
	return FaultState{
		Crashed:   false,
		DropRate:  0,
		DelayMs:   0,
		Partition: nil,
	}
}

// Details related to cluster
type Node struct {
	ID       string     `json:"id"`
	Address  string     `json:"address"`
	Port     int        `json:"port"`
	LastSeen time.Time  `json:"lastSeen"`
	Status   string     `json:"status,omitempty"` // "active" or "crashed"
	Fault    FaultState `json:"fault"`
}

type Cluster struct {
	ID       string
	Protocol string
	Nodes    map[string]*Node
}
