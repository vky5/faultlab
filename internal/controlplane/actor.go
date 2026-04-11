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
	"syscall"
	"time"

	"github.com/vky5/faultlab/internal/cluster"
	controlplanesvc "github.com/vky5/faultlab/internal/controlplane/service"
	"github.com/vky5/faultlab/internal/experiment"
	pb "github.com/vky5/faultlab/internal/protocol"
)

type ActorOptions struct {
	ProjectRoot    string
	DefaultCPHost  string
	DefaultCPPort  int
	NodeBinaryPath string
}

type managedNodeProcess struct {
	NodeID    string `json:"nodeId"`
	ClusterID string `json:"clusterId"`
	PID       int    `json:"pid"`
}

type Actor struct {
	service   *controlplanesvc.Service
	options   ActorOptions // options for node process management and defaults for command execution
	cmdCh     chan Command
	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.Mutex
	nodeProcs map[string]*exec.Cmd
	nodeMeta  map[string]string
}

// Need service for node verification registration
func NewActor(service *controlplanesvc.Service, options ActorOptions) *Actor {
	ctx, cancel := context.WithCancel(context.Background())

	return &Actor{
		service:   service,
		options:   options,
		cmdCh:     make(chan Command, 32),
		ctx:       ctx,
		cancel:    cancel,
		nodeProcs: make(map[string]*exec.Cmd),
		nodeMeta:  make(map[string]string),
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
	a.stopAllManagedNodeProcesses()
	if a.cancel != nil {
		a.cancel()
	}
}

func (a *Actor) stopAllManagedNodeProcesses() {
	type processRef struct {
		nodeID string
		proc   *exec.Cmd
	}

	a.mu.Lock()
	procs := make([]processRef, 0, len(a.nodeProcs))
	for nodeID, proc := range a.nodeProcs {
		procs = append(procs, processRef{nodeID: nodeID, proc: proc})
	}
	a.mu.Unlock()

	for _, p := range procs {
		if p.proc == nil || p.proc.Process == nil {
			continue
		}
		if err := killProcessTree(p.proc); err != nil {
			log.Printf("failed stopping managed node process during actor stop: node=%s err=%v", p.nodeID, err)
		}
	}

	a.mu.Lock()
	for _, p := range procs {
		delete(a.nodeProcs, p.nodeID)
		delete(a.nodeMeta, p.nodeID)
	}
	a.mu.Unlock()
}

// Bridges directly to the service for SSE log streaming (does not use command actor channel loop)
func (a *Actor) SubscribeLogs() chan *pb.LogRequest {
	return a.service.SubscribeLogs()
}

func (a *Actor) UnsubscribeLogs(ch chan *pb.LogRequest) {
	a.service.UnsubscribeLogs(ch)
}

// cmdCh takes input from from a.Submit(cmd)
// runs in main.go in separate goroutine that handles all incoming commands
// single executionre point for the incoming commands
func (a *Actor) Run() {
	for {
		select {
		case <-a.ctx.Done():
			return
		case cmd := <-a.cmdCh:
			switch cmd.Type {

			case CmdCreateCluster:
				err := a.service.CreateCluster(cmd.ClusterID, cmd.Protocol)
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

			case CmdStopNodeProcess:
				a.mu.Lock()
				proc, ok := a.nodeProcs[cmd.NodeID]
				a.mu.Unlock()
				if !ok {
					cmd.Reply(nil, fmt.Errorf("node process not running: %s", cmd.NodeID))
					continue
				}

				if proc.Process == nil {
					cmd.Reply(nil, fmt.Errorf("node process has no process handle: %s", cmd.NodeID))
					continue
				}

				if err := killProcessTree(proc); err != nil {
					cmd.Reply(nil, fmt.Errorf("failed to stop node process %s: %w", cmd.NodeID, err))
					continue
				}

				a.mu.Lock()
				delete(a.nodeProcs, cmd.NodeID)
				delete(a.nodeMeta, cmd.NodeID)
				a.mu.Unlock()

				cmd.Reply(map[string]any{"stopped": true, "nodeId": cmd.NodeID}, nil)

			case CmdListNodeProcesses:
				a.mu.Lock()
				procs := make([]managedNodeProcess, 0, len(a.nodeProcs))
				for nodeID, proc := range a.nodeProcs {
					pid := 0
					if proc != nil && proc.Process != nil {
						pid = proc.Process.Pid
					}
					procs = append(procs, managedNodeProcess{NodeID: nodeID, ClusterID: a.nodeMeta[nodeID], PID: pid})
				}
				a.mu.Unlock()

				cmd.Reply(procs, nil)

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
				nodes, err := a.service.GetNodes(cmd.ClusterID)
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
				clusterIDs := a.service.GetClusters()

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
					nodes, err := a.service.GetNodes(id)
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
					if c, err := a.service.GetCluster(id); err == nil {
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
				n, err := a.service.GetNode(cmd.ClusterID, cmd.NodeID)
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
				n, err := a.service.GetNode(cmd.ClusterID, cmd.NodeID)
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
				n, err := a.service.GetNode(cmd.ClusterID, cmd.NodeID)
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
				n, err := a.service.GetNode(cmd.ClusterID, cmd.NodeID)
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
					n, err := a.service.GetNode(cmd.ClusterID, targetNodeID)
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
				a.service.RecordWriteKeyForMetrics(cmd.ClusterID, cmd.Key)
				cmd.Reply(map[string]any{"status": "ok"}, nil)

			case CmdKVGet:
				result, err := a.service.ExecuteKVGet(a.ctx, cmd.ClusterID, cmd.NodeID, cmd.Key)
				if err != nil {
					log.Println("kv-get error:", err)
					cmd.Reply(nil, err)
					continue
				}

				response := map[string]any{"value": result.Value}
				if result.HasMetadata {
					response["version"] = result.Version
					response["origin"] = result.Origin
				}

				cmd.Reply(response, nil)

			case CmdSetClusterProtocol:
				err := a.service.SetClusterProtocol(a.ctx, cmd.ClusterID, cmd.Protocol)
				if err != nil {
					log.Println("set-protocol error:", err)
					cmd.Reply(nil, err)
					continue
				}
				cmd.Reply(map[string]any{"status": "ok"}, nil)

			case CmdKillCluster:
				stopped, err := a.killCluster(cmd.ClusterID)
				if err != nil {
					cmd.Reply(nil, err)
					continue
				}
				cmd.Reply(map[string]any{"status": "ok", "clusterId": cmd.ClusterID, "stoppedNodeProcesses": stopped}, nil)

			case CmdMetricsStart:
				interval := time.Duration(cmd.IntervalMs) * time.Millisecond
				if err := a.service.StartMetricsSession(cmd.ClusterID, interval); err != nil {
					cmd.Reply(nil, err)
					continue
				}
				effectiveInterval := cmd.IntervalMs
				if effectiveInterval <= 0 {
					effectiveInterval = 1000
				}
				cmd.Reply(map[string]any{"status": "started", "clusterId": cmd.ClusterID, "intervalMs": effectiveInterval}, nil)

			case CmdMetricsStop:
				result, err := a.service.StopMetricsSession(cmd.ClusterID)
				if err != nil {
					cmd.Reply(nil, err)
					continue
				}
				cmd.Reply(result, nil)

			case CmdMetricsWatchKey:
				if err := a.service.AddMetricsWatchKey(cmd.ClusterID, cmd.Key); err != nil {
					cmd.Reply(nil, err)
					continue
				}
				cmd.Reply(map[string]any{"status": "watching", "clusterId": cmd.ClusterID, "key": cmd.Key}, nil)

			case CmdMetricsShow:
				result, err := a.service.GetMetricsSnapshot(cmd.ClusterID)
				if err != nil {
					cmd.Reply(nil, err)
					continue
				}
				cmd.Reply(result, nil)

			case CmdRunHypothesis:
				started, err := a.runHypothesis(cmd)
				if err != nil {
					cmd.Reply(nil, err)
					continue
				}
				cmd.Reply(started, nil)

			case CmdHelp:
				help := []string{
					"start-node <node-id> <port> [--cluster-id <id>] [--host <host>] [--peers <csv>] [--cp-host <host>] [--cp-port <port>]",
					"stop-node <node-id>",
					"list-node-procs",
					"new-cluster <cluster-id> [--protocol <gossip|raft>] (default: gossip)",
					"add-node <cluster-id> <node-id> <host> <port>",
					"remove-node <cluster-id> <node-id>",
					"list-nodes <cluster-id>",
					"list-clusters",
					"set-protocol <cluster-id> <gossip|raft>",
					"kill-cluster <cluster-id>",
					"kv-put <cluster-id> <node-id> <key> <value>",
					"kv-get <cluster-id> <node-id> <key>",
					"run-hypothesis <relative-experiment-path>",
					"metrics-start <cluster-id> [interval-ms]",
					"metrics-watch-key <cluster-id> <key>",
					"metrics-show <cluster-id>",
					"metrics-stop <cluster-id>",
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
	proc.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	proc.Dir = projectRoot
	proc.Stdout = os.Stdout
	proc.Stderr = os.Stderr

	if err := proc.Start(); err != nil {
		return fmt.Errorf("failed to start node process %s: %w", cmd.NodeID, err)
	}

	a.mu.Lock()
	a.nodeProcs[cmd.NodeID] = proc
	a.nodeMeta[cmd.NodeID] = clusterID
	a.mu.Unlock()

	go func() {
		if err := proc.Wait(); err != nil {
			log.Printf("node process %s exited with error: %v", cmd.NodeID, err)
		}
		a.mu.Lock()
		delete(a.nodeProcs, cmd.NodeID)
		delete(a.nodeMeta, cmd.NodeID)
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

func (a *Actor) killCluster(clusterID string) (int, error) {
	clusterID = strings.TrimSpace(clusterID)
	if clusterID == "" {
		return 0, fmt.Errorf("cluster id is required")
	}

	a.service.StopMetricsSessionIfActive(clusterID)

	type processRef struct {
		nodeID string
		proc   *exec.Cmd
	}

	a.mu.Lock()
	toKill := make([]processRef, 0)
	for nodeID, proc := range a.nodeProcs {
		if a.nodeMeta[nodeID] != clusterID {
			continue
		}
		toKill = append(toKill, processRef{nodeID: nodeID, proc: proc})
	}
	a.mu.Unlock()

	stopped := 0
	for _, item := range toKill {
		if item.proc == nil || item.proc.Process == nil {
			continue
		}
		if err := killProcessTree(item.proc); err != nil {
			return stopped, fmt.Errorf("failed to stop node process %s: %w", item.nodeID, err)
		}
		stopped++
	}

	a.mu.Lock()
	for _, item := range toKill {
		delete(a.nodeProcs, item.nodeID)
		delete(a.nodeMeta, item.nodeID)
	}
	a.mu.Unlock()

	if _, err := a.service.GetCluster(clusterID); err == nil {
		if err := a.service.RemoveCluster(clusterID); err != nil {
			return stopped, err
		}
	}

	if stopped == 0 {
		return 0, fmt.Errorf("no managed node processes found for cluster %s", clusterID)
	}

	return stopped, nil
}

func (a *Actor) runHypothesis(cmd Command) (map[string]any, error) {
	relOrAbsPath := strings.TrimSpace(cmd.FilePath)
	if relOrAbsPath == "" {
		return nil, fmt.Errorf("hypothesis file path is required")
	}

	projectRoot := strings.TrimSpace(cmd.ProjectRoot)
	if projectRoot == "" {
		projectRoot = strings.TrimSpace(a.options.ProjectRoot)
	}
	if projectRoot == "" {
		projectRoot = "."
	}

	absPath := relOrAbsPath
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(projectRoot, relOrAbsPath)
	}

	exp, err := experiment.LoadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("load hypothesis: %w", err)
	}
	clusterID := strings.TrimSpace(exp.Cluster.ID)
	if clusterID == "" {
		return nil, fmt.Errorf("cluster.id is required in hypothesis yaml")
	}

	compileOpts := experiment.CompileOptions{
		ClusterID:        clusterID,
		ControlPlaneHost: a.options.DefaultCPHost, // these two are required because if we are starting a node we need to inform it about controlplane host and port so that it can send heartbeats and register itself
		ControlPlanePort: a.options.DefaultCPPort,
	}
	batches, err := exp.Compile(compileOpts)
	if err != nil {
		return nil, fmt.Errorf("compile hypothesis: %w", err)
	}
	trackedKeys := collectHypothesisWriteKeys(exp)

	go a.executeHypothesis(exp.Name, clusterID, absPath, batches, trackedKeys)

	return map[string]any{
		"status":       "started",
		"name":         exp.Name,
		"clusterId":    clusterID,
		"path":         relOrAbsPath,
		"resolvedPath": absPath,
		"steps":        len(batches),
	}, nil
}

func collectHypothesisWriteKeys(exp *experiment.Experiment) []string {
	if exp == nil {
		return nil
	}

	seen := map[string]struct{}{}
	keys := make([]string, 0)
	for _, step := range exp.Timeline {
		if strings.TrimSpace(step.Action) != "write" {
			continue
		}
		key := strings.TrimSpace(step.Key)
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}

	return keys
}

func killProcessTree(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return fmt.Errorf("process is not running")
	}

	pid := cmd.Process.Pid
	if pid <= 0 {
		return fmt.Errorf("invalid process id")
	}

	// Kill the full process group so `go run` parent and spawned node child both exit.
	if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
		if err == syscall.ESRCH {
			return nil
		}
		return err
	}

	return nil
}

func (a *Actor) executeHypothesis(name, clusterID, path string, batches []experiment.CommandBatch, trackedKeys []string) {
	started := time.Now()
	metricsStarted := false
	watchRegistered := false

	defer func() {
		if !metricsStarted {
			return
		}
		result, err := a.service.StopMetricsSession(clusterID)
		if err != nil {
			log.Printf("hypothesis %s metrics stop failed for cluster %s: %v", name, clusterID, err)
			return
		}
		log.Printf("hypothesis %s metrics finalized for cluster %s: %+v", name, clusterID, result)
	}()

	for i, batch := range batches {
		waitFor := started.Add(batch.At)
		if delay := time.Until(waitFor); delay > 0 {
			time.Sleep(delay)
		}

		if !metricsStarted {
			startedNow, err := a.service.StartMetricsSessionIfInactive(clusterID, time.Second)
			if err == nil {
				metricsStarted = startedNow || a.service.Metrics().IsActive(clusterID)
				if metricsStarted {
					log.Printf("hypothesis %s metrics started for cluster %s", name, clusterID)
				}
			} else {
				log.Printf("hypothesis %s metrics start deferred for cluster %s: %v", name, clusterID, err)
			}
		}

		if metricsStarted && !watchRegistered {
			allWatched := true
			for _, key := range trackedKeys {
				if err := a.service.AddMetricsWatchKey(clusterID, key); err != nil {
					allWatched = false
					log.Printf("hypothesis %s metrics watch deferred for cluster %s key %s: %v", name, clusterID, key, err)
					break
				}
			}
			if allWatched {
				watchRegistered = true
			}
		}

		for _, raw := range batch.Commands {
			if strings.HasPrefix(strings.TrimSpace(raw), "run-hypothesis") || strings.HasPrefix(strings.TrimSpace(raw), "run-experiment") {
				log.Printf("hypothesis %s skipped nested hypothesis command at step %d: %s", name, i, raw)
				continue
			}

			if _, err := ExecuteRuntimeCommand(raw, a); err != nil {
				log.Printf("hypothesis %s (cluster %s) failed at step %d command %q: %v", name, clusterID, i, raw, err)
				return
			}
		}
	}

	log.Printf("hypothesis %s (cluster %s) finished (%s)", name, clusterID, path)
}

/*
actor -> service -> NodeClient -> Node gRPC API
*/
