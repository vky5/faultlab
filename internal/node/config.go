package node

// Details related to individual node and its peers
type NodeConfig struct {
	ID    string
	Port  int
	Peers []Peer
}

type Peer struct {
	ID   string
	Port int
}
