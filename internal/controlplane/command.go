package controlplane

type CommandType int

const (
	CmdCreateCluster CommandType = iota
	CmdStartNodeProcess
	CmdStopNodeProcess
	CmdListNodeProcesses
	CmdAddNode
	CmdRemoveNode
	CmdListNodes
	CmdListClusters
	CmdSetFaultParams
	CmdCrashNode
	CmdRecoverNode
	CmdSetDropRate
	CmdSetDelay
	CmdSetPartition
	CmdKVPut
	CmdKVGet
	CmdSetClusterProtocol
	CmdRunHypothesis
	CmdKillCluster
	CmdMetricsStart
	CmdMetricsStop
	CmdMetricsWatchKey
	CmdMetricsShow
	CmdHelp
)

type CommandResult struct {
	Data  interface{}
	Error error
}

type Command struct {
	Type        CommandType
	ClusterID   string
	Protocol    string
	NodeID      string
	Key         string
	Value       string
	Host        string
	Port        int
	Crashed     bool
	DropRate    float64
	DelayMs     int
	Partition   []string
	PeerID      string
	PeersCSV    string
	Enabled     bool
	FilePath    string
	ProjectRoot string
	CPHost      string
	CPPort      int
	IntervalMs  int
	replyCh     chan CommandResult
}

// Init command with a reply channel
func NewCommand(t CommandType) Command {
	return Command{
		Type:    t,
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
