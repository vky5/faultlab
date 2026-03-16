package cluster


import "time"

// Details related to cluster
type Node struct {
	ID       string
	Address  string
	Port     int
	LastSeen time.Time
}

type Cluster struct {
	ID       string
	Protocol string
	Nodes    map[string]*Node
}