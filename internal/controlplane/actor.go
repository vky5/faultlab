package controlplane

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/vky5/faultlab/internal/cluster"
	clustermanager "github.com/vky5/faultlab/internal/cluster/manager"
	controlplanesvc "github.com/vky5/faultlab/internal/controlplane/service"
	pb "github.com/vky5/faultlab/internal/protocol"
)

type ActorOptions struct {
	ProjectRoot    string
	DefaultCPHost  string
	DefaultCPPort  int
	NodeBinaryPath string
}

type managedNodeProcess struct {
	NodeID string `json:"nodeId"`
	PID    int    `json:"pid"`
}

type Actor struct {
	manager   *clustermanager.Manager
	service   *controlplanesvc.Service
	options   ActorOptions
	cmdCh     chan Command
	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.Mutex
	nodeProcs map[string]*exec.Cmd
}

// Need service for node verification registration
func NewActor(manager *clustermanager.Manager, service *controlplanesvc.Service, options ActorOptions) *Actor {
	ctx, cancel := context.WithCancel(context.Background())

	return &Actor{
		manager:   manager,
		service:   service,
		options:   options,
		cmdCh:     make(chan Command, 32),
		ctx:       ctx,
		cancel:    cancel,
		nodeProcs: make(map[string]*exec.Cmd),
	}
}

// taking input of commands in command channel
func (a *Actor) Submit(cmd Command) {
	select {
	case <-a.ctx.Done():
		return
	case a.cmdCh <- cmd:
	}
}

// Stop cancels actor processing and causes Run() to exit.
func (a *Actor) Stop() {
	if a.cancel != nil {
		a.cancel()
	}
}

// Bridges directly to the service for SSE log streaming (does not use command actor channel loop)
func (a *Actor) SubscribeLogs() chan *pb.LogRequest {
	return a.service.SubscribeLogs()
}

func (a *Actor) UnsubscribeLogs(ch chan *pb.LogRequest) {
	a.service.UnsubscribeLogs(ch)
}

// single executionre point for the incoming commands
func (a *Actor) Run() {
	for {
		select {
		case <-a.ctx.Done():
			return
		case cmd := <-a.cmdCh:
			switch cmd.Type {

			case CmdCreateCluster:
				err := a.manager.CreateCluster(cmd.ClusterID, cmd.Protocol)
				if err != nil {
					fmt.Println("create cluster error:", err)
					cmd.Reply(nil, err)
					continue
				}
				fmt.Println("cluster created:", cmd.ClusterID)
				cmd.Reply(nil, nil)

			case CmdStartNodeProcess:
				err := a.startNodeProcess(cmd)
				cmd.Reply(map[string]any{"started": err == nil, "nodeId": cmd.NodeID}, err)

			case CmdAddNode:
				// Run Verification + Registration
				err := a.service.RegisterNode(a.ctx, cmd.ClusterID, cmd.NodeID, cmd.Host, cmd.Port)
				if err != nil {
					log.Println("add node error:", err)
					cmd.Reply(nil, err)
					continue
				}
				cmd.Reply(nil, nil)

			case CmdRemoveNode:
				err := a.service.RemoveNode(a.ctx, cmd.ClusterID, cmd.NodeID)
				if err != nil {
					fmt.Println("remove node error:", err)
					cmd.Reply(nil, err)
					continue
				}
				cmd.Reply(nil, nil)

			case CmdListNodes:
				nodes, err := a.manager.GetNodes(cmd.ClusterID)
				if err != nil {
					fmt.Println("list error:", err)
					cmd.Reply(nil, err)
					continue
				}

				for _, n := range nodes {
					fmt.Println(n.ID, n.Address, n.Port)
				}
				cmd.Reply(nodes, nil)

			case CmdListClusters:
				clusterIDs := a.manager.GetClusters()

				type NodeInfo struct {
					ID                  string                         `json:"id"`
					Address             string                         `json:"address"`
					Port                int                            `json:"port"`
					Status              string                         `json:"status,omitempty"`
					Fault               cluster.FaultState             `json:"fault"`
					ActiveProtocolKey   string                         `json:"activeProtocolKey,omitempty"`
					ActiveProtocolEpoch uint64                         `json:"activeProtocolEpoch,omitempty"`
					Capabilities        cluster.NodeActionCapabilities `json:"capabilities"`
					CapabilitiesAt      int64                          `json:"capabilitiesAt,omitempty"`
				}
				type ClusterInfo struct {
					ID       string     `json:"id"`
					Protocol string     `json:"protocol"`
					Nodes    []NodeInfo `json:"nodes"`
				}

				resp := []ClusterInfo{}
				for _, id := range clusterIDs {
					nodes, err := a.manager.GetNodes(id)
					if err != nil {
						continue
					}

					var ni []NodeInfo
					for _, node := range nodes {
						ni = append(ni, NodeInfo{
							ID:                  node.ID,
							Address:             node.Address,
							Port:                node.Port,
							Status:              node.Status,
							Fault:               node.Fault,
							ActiveProtocolKey:   node.ActiveProtocolKey,
							ActiveProtocolEpoch: node.ActiveProtocolEpoch,
							Capabilities:        node.Capabilities,
							CapabilitiesAt:      node.CapabilitiesAt,
						})
					}

					var protocol string
					if c, err := a.manager.GetCluster(id); err == nil {
						protocol = c.Protocol
					}

					resp = append(resp, ClusterInfo{ID: id, Protocol: protocol, Nodes: ni})
				}
				cmd.Reply(resp, nil)

			case CmdSetFaultParams:
				fault := cluster.FaultState{
					Crashed:   cmd.Crashed,
					DropRate:  cmd.DropRate,
					DelayMs:   cmd.DelayMs,
					Partition: cmd.Partition,
				}

				err := a.service.SetFaultParams(cmd.ClusterID, cmd.NodeID, fault)
				if err != nil {
					log.Println("set fault params error:", err)
					cmd.Reply(nil, err)
					continue
				}
				cmd.Reply(map[string]any{"status": "ok"}, nil)

			case CmdCrashNode:
				n, err := a.manager.GetNode(cmd.ClusterID, cmd.NodeID)
				if err != nil {
					cmd.Reply(nil, err)
					continue
				}
				fault := n.Fault
				fault.Crashed = true
				if err := a.service.SetFaultParams(cmd.ClusterID, cmd.NodeID, fault); err != nil {
					cmd.Reply(nil, err)
					continue
				}
				cmd.Reply(map[string]any{"status": "ok"}, nil)

			case CmdRecoverNode:
				n, err := a.manager.GetNode(cmd.ClusterID, cmd.NodeID)
				if err != nil {
					cmd.Reply(nil, err)
					continue
				}
				fault := n.Fault
				fault.Crashed = false
				if err := a.service.SetFaultParams(cmd.ClusterID, cmd.NodeID, fault); err != nil {
					cmd.Reply(nil, err)
					continue
				}
				cmd.Reply(map[string]any{"status": "ok"}, nil)

			case CmdSetDropRate:
				n, err := a.manager.GetNode(cmd.ClusterID, cmd.NodeID)
				if err != nil {
					cmd.Reply(nil, err)
					continue
				}
				fault := n.Fault
				fault.DropRate = cmd.DropRate
				if err := a.service.SetFaultParams(cmd.ClusterID, cmd.NodeID, fault); err != nil {
					cmd.Reply(nil, err)
					continue
				}
				cmd.Reply(map[string]any{"status": "ok"}, nil)

			case CmdSetDelay:
				n, err := a.manager.GetNode(cmd.ClusterID, cmd.NodeID)
				if err != nil {
					cmd.Reply(nil, err)
					continue
				}
				fault := n.Fault
				fault.DelayMs = cmd.DelayMs
				if err := a.service.SetFaultParams(cmd.ClusterID, cmd.NodeID, fault); err != nil {
					cmd.Reply(nil, err)
					continue
				}
				cmd.Reply(map[string]any{"status": "ok"}, nil)

			case CmdSetPartition:
				// Helper to update partition list for a single node
				updateNodePartition := func(targetNodeID, otherPeerID string, enabled bool) error {
					n, err := a.manager.GetNode(cmd.ClusterID, targetNodeID)
					if err != nil {
						return err
					}
					fault := n.Fault
					next := make([]string, 0, len(fault.Partition)+1)
					seen := false
					for _, peer := range fault.Partition {
						if peer == otherPeerID {
							seen = true
							if enabled {
								next = append(next, peer)
							}
							continue
						}
						next = append(next, peer)
					}
					if enabled && !seen {
						next = append(next, otherPeerID)
					}
					fault.Partition = next
					return a.service.SetFaultParams(cmd.ClusterID, targetNodeID, fault)
				}

				// 1. Update the requested node (Node A -> Node B)
				if err := updateNodePartition(cmd.NodeID, cmd.PeerID, cmd.Enabled); err != nil {
					log.Printf("symmetric partition failed for Node A (%s): %v", cmd.NodeID, err)
					cmd.Reply(nil, err)
					continue
				}

				// 2. Symmetric update: Update the peer (Node B -> Node A)
				if err := updateNodePartition(cmd.PeerID, cmd.NodeID, cmd.Enabled); err != nil {
					log.Printf("symmetric partition failed for Node B (%s): %v", cmd.PeerID, err)
					// We continue even if Node B failed, but we report Node A's success in state
				}

				cmd.Reply(map[string]any{"status": "ok", "mode": "symmetric"}, nil)

			case CmdKVPut:
				err := a.service.ExecuteKVPut(a.ctx, cmd.ClusterID, cmd.NodeID, cmd.Key, cmd.Value)
				if err != nil {
					log.Println("kv-put error:", err)
					cmd.Reply(nil, err)
					continue
				}
				cmd.Reply(map[string]any{"status": "ok"}, nil)

			case CmdKVGet:
				value, err := a.service.ExecuteKVGet(a.ctx, cmd.ClusterID, cmd.NodeID, cmd.Key)
				if err != nil {
					log.Println("kv-get error:", err)
					cmd.Reply(nil, err)
					continue
				}
				cmd.Reply(map[string]any{"value": value}, nil)

			case CmdSetClusterProtocol:
				err := a.service.SetClusterProtocol(a.ctx, cmd.ClusterID, cmd.Protocol)
				if err != nil {
					log.Println("set-protocol error:", err)
					cmd.Reply(nil, err)
					continue
				}
				cmd.Reply(map[string]any{"status": "ok"}, nil)

			case CmdHelp:
				help := []string{
					"start-node <node-id> <port> [--cluster-id <id>] [--host <host>] [--peers <csv>] [--cp-host <host>] [--cp-port <port>]",
					"new-cluster <cluster-id> [--protocol <gossip|raft>] (default: gossip)",
					"add-node <cluster-id> <node-id> <host> <port>",
					"remove-node <cluster-id> <node-id>",
					"list-nodes <cluster-id>",
					"list-clusters",
					"set-protocol <cluster-id> <gossip|raft>",
					"kv-put <cluster-id> <node-id> <key> <value>",
					"kv-get <cluster-id> <node-id> <key>",
					"set-fault <cluster-id> <node-id> <crashed:true|false> <drop-rate:0..1> <delay-ms:int> [partition-csv]",
					"fault-crash <cluster-id> <node-id>",
					"fault-recover <cluster-id> <node-id>",
					"fault-drop <cluster-id> <node-id> <drop-rate:0..1>",
					"fault-delay <cluster-id> <node-id> <delay-ms:int>",
					"fault-partition <cluster-id> <node-id> <peer-id> <enabled:true|false>",
					"help",
				}
				cmd.Reply(help, nil)
			}
		}
	}

}

func (a *Actor) startNodeProcess(cmd Command) error {
	a.mu.Lock()
	if _, exists := a.nodeProcs[cmd.NodeID]; exists {
		a.mu.Unlock()
		return fmt.Errorf("node process already running: %s", cmd.NodeID)
	}
	a.mu.Unlock()

	projectRoot := cmd.ProjectRoot
	if projectRoot == "" {
		projectRoot = a.options.ProjectRoot
	}
	if projectRoot == "" {
		projectRoot = "."
	}
	cpHost := cmd.CPHost
	if cpHost == "" {
		cpHost = a.options.DefaultCPHost
	}
	if cpHost == "" {
		cpHost = "localhost"
	}
	cpPort := cmd.CPPort
	if cpPort == 0 {
		cpPort = a.options.DefaultCPPort
	}
	if cpPort == 0 {
		cpPort = 9000
	}

	clusterID := cmd.ClusterID
	if clusterID == "" {
		clusterID = "default"
	}
	host := cmd.Host
	if host == "" {
		host = "localhost"
	}

	nodeArgs := []string{
		"-id", cmd.NodeID,
		"-port", fmt.Sprintf("%d", cmd.Port),
		"-cluster-id", clusterID,
		"-host", host,
		"-cp-host", cpHost,
		"-cp-port", fmt.Sprintf("%d", cpPort),
	}
	if cmd.PeersCSV != "" {
		nodeArgs = append(nodeArgs, "-peers", cmd.PeersCSV)
	}

	proc := a.newNodeProcess(projectRoot, nodeArgs)
	proc.Dir = projectRoot
	proc.Stdout = os.Stdout
	proc.Stderr = os.Stderr

	if err := proc.Start(); err != nil {
		return fmt.Errorf("failed to start node process %s: %w", cmd.NodeID, err)
	}

	a.mu.Lock()
	a.nodeProcs[cmd.NodeID] = proc
	a.mu.Unlock()

	go func() {
		if err := proc.Wait(); err != nil {
			log.Printf("node process %s exited with error: %v", cmd.NodeID, err)
		}
		a.mu.Lock()
		delete(a.nodeProcs, cmd.NodeID)
		a.mu.Unlock()
	}()

	log.Printf("controlplane actor started node process %s with pid %d", cmd.NodeID, proc.Process.Pid)
	return nil
}

func (a *Actor) newNodeProcess(projectRoot string, nodeArgs []string) *exec.Cmd {
	binaryPath := a.options.NodeBinaryPath
	if strings.TrimSpace(binaryPath) == "" {
		binaryPath = "bin/node"
	}

	if !filepath.IsAbs(binaryPath) {
		binaryPath = filepath.Join(projectRoot, binaryPath)
	}

	if info, err := os.Stat(binaryPath); err == nil && !info.IsDir() {
		if info.Mode()&0o111 != 0 {
			return exec.CommandContext(a.ctx, binaryPath, nodeArgs...)
		}
		log.Printf("node binary exists but is not executable: %s; falling back to go run", binaryPath)
	} else {
		log.Printf("node binary not found at %s; falling back to go run", binaryPath)
	}

	goArgs := append([]string{"run", "./cmd/node"}, nodeArgs...)
	return exec.CommandContext(a.ctx, "go", goArgs...)
}

/*
actor -> service -> NodeClient -> Node gRPC API
*/
