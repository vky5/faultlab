package controlplane

type CommandType int

const (
	CmdCreateCluster CommandType = iota
	CmdAddNode
	CmdRemoveNode 
	CmdListNodes
	CmdListClusters
)

type CommandResult struct {
	Data  interface{}
	Error error
}

type Command struct{
	Type CommandType
	ClusterID string
	Protocol string
	NodeID string
	Host string
	Port int
	replyCh chan CommandResult
}

// Init command with a reply channel
func NewCommand(t CommandType) Command {
	return Command{
		Type: t,
		replyCh: make(chan CommandResult, 1),
	}
}

func (c *Command) Reply(data interface{}, err error) {
	if c.replyCh != nil {
		c.replyCh <- CommandResult{Data: data, Error: err}
	}
}

func (c *Command) MapWait() (interface{}, error) {
	if c.replyCh == nil {
		return nil, nil
	}
	res := <-c.replyCh
	return res.Data, res.Error
}
