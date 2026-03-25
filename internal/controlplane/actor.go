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
}

// Need service for node verification registration
func NewActor(manager *clustermanager.Manager, service *controlplanesvc.Service) *Actor {
	return &Actor{
		manager: manager,
		service: service,
		cmdCh:   make(chan Command, 32),
	}
}

// taking input of commands in command channel
func (a *Actor) Submit(cmd Command) {
	a.cmdCh <- cmd
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
	for cmd := range a.cmdCh {
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
			err := a.service.RegisterNode(context.Background(), cmd.ClusterID, cmd.NodeID, cmd.Host, cmd.Port)
			if err != nil {
				log.Println("add node error:", err)
				cmd.Reply(nil, err)
				continue
			}
			cmd.Reply(nil, nil)

		case CmdRemoveNode:
			err := a.service.RemoveNode(context.Background(), cmd.ClusterID, cmd.NodeID)
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
				ID      string `json:"id"`
				Address string `json:"address"`
				Port    int    `json:"port"`
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
					ni = append(ni, NodeInfo{ID: node.ID, Address: node.Address, Port: node.Port})
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

		case CmdHelp:
			help := []string{
				"new-cluster <cluster-id> [protocol]",
				"add-node <cluster-id> <node-id> <host> <port>",
				"remove-node <cluster-id> <node-id>",
				"list-nodes <cluster-id>",
				"list-clusters",
				"set-fault <cluster-id> <node-id> <crashed:true|false> <drop-rate:0..1> <delay-ms:int> [partition-csv]",
				"help",
			}
			cmd.Reply(help, nil)
		}
	}
}
