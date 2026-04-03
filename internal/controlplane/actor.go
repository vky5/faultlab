package controlplane

import (
	"context"
	"fmt"
	"log"

	"github.com/vky5/faultlab/internal/cluster"
	clustermanager "github.com/vky5/faultlab/internal/cluster/manager"
	controlplanesvc "github.com/vky5/faultlab/internal/controlplane/service"
	pb "github.com/vky5/faultlab/internal/protocol"
)

type Actor struct {
	manager *clustermanager.Manager
	service *controlplanesvc.Service
	cmdCh   chan Command
	ctx     context.Context
	cancel  context.CancelFunc
}

// Need service for node verification registration
func NewActor(manager *clustermanager.Manager, service *controlplanesvc.Service) *Actor {
	ctx, cancel := context.WithCancel(context.Background())

	return &Actor{
		manager: manager,
		service: service,
		cmdCh:   make(chan Command, 32),
		ctx:     ctx,
		cancel:  cancel,
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
					ID      string             `json:"id"`
					Address string             `json:"address"`
					Port    int                `json:"port"`
					Status  string             `json:"status,omitempty"`
					Fault   cluster.FaultState `json:"fault"`
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
							ID:      node.ID,
							Address: node.Address,
							Port:    node.Port,
							Status:  node.Status,
							Fault:   node.Fault,
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
				n, err := a.manager.GetNode(cmd.ClusterID, cmd.NodeID)
				if err != nil {
					cmd.Reply(nil, err)
					continue
				}
				fault := n.Fault
				next := make([]string, 0, len(fault.Partition)+1)
				seen := false
				for _, peer := range fault.Partition {
					if peer == cmd.PeerID {
						seen = true
						if cmd.Enabled {
							next = append(next, peer)
						}
						continue
					}
					next = append(next, peer)
				}
				if cmd.Enabled && !seen {
					next = append(next, cmd.PeerID)
				}
				fault.Partition = next
				if err := a.service.SetFaultParams(cmd.ClusterID, cmd.NodeID, fault); err != nil {
					cmd.Reply(nil, err)
					continue
				}
				cmd.Reply(map[string]any{"status": "ok"}, nil)

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

			case CmdHelp:
				help := []string{
					"new-cluster <cluster-id> [--protocol <gossip|raft>] (default: gossip)",
					"add-node <cluster-id> <node-id> <host> <port>",
					"remove-node <cluster-id> <node-id>",
					"list-nodes <cluster-id>",
					"list-clusters",
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

/*
actor -> service -> NodeClient -> Node gRPC API
*/
