package controlplane

type CommandType int

const (
	CmdCreateCluster CommandType = iota
	CmdRemoveNode 
	CmdListNodes
)

type Command struct{
	Type CommandType
	ClusterID string
	NodeID string
	Host string
	Port int
}

