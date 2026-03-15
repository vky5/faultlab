package controlplane

import (
	"fmt"

	clustermanager "github.com/vky5/faultlab/internal/cluster/manager"
)

type Actor struct {
	manager *clustermanager.Manager
	cmdCh   chan Command
}

func NewActor(manager *clustermanager.Manager) *Actor {
	return &Actor{
		manager: manager,
		cmdCh:   make(chan Command, 32),
	}
}

// taking input of commands in command channel
func (a *Actor) Submit(cmd Command) {
	a.cmdCh <- cmd
}

// single executionre point for the incoming commands
func (a *Actor) Run() {

	for cmd := range a.cmdCh {

		switch cmd.Type {

		case CmdCreateCluster:
			err := a.manager.CreateCluster(cmd.ClusterID)
			if err != nil {
				fmt.Println("create cluster error:", err)
				continue
			}
			fmt.Println("cluster created:", cmd.ClusterID)

		case CmdRemoveNode:
			err := a.manager.RemoveNode(cmd.ClusterID, cmd.NodeID)
			if err != nil {
				fmt.Println("remove node error:", err)
			}

		case CmdListNodes:
			nodes, err := a.manager.GetNodes(cmd.ClusterID)
			if err != nil {
				fmt.Println("list error:", err)
				continue
			}

			for _, n := range nodes {
				fmt.Println(n.ID, n.Address, n.Port)
			}
		}
	}
}
