package controlplane

import (
	"fmt"
	"strconv"
	"strings"
)

func Parse(input string) (Command, error) {

	parts := strings.Fields(input)

	if len(parts) == 0 {
		return Command{}, fmt.Errorf("empty command")
	}

	switch parts[0] {

	case "new-cluster":
		protocol := "gossip"
		if len(parts) >= 3 {
			protocol = parts[2]
		}
		return Command{
			Type:      CmdCreateCluster,
			ClusterID: parts[1],
			Protocol:  protocol,
		}, nil

	case "remove-node":
		return Command{
			Type:      CmdRemoveNode,
			ClusterID: parts[1],
			NodeID:    parts[2],
		}, nil

	case "list-nodes":
		return Command{
			Type:      CmdListNodes,
			ClusterID: parts[1],
		}, nil

	case "add-node":
		port, _ := strconv.Atoi(parts[4])

		return Command{
			Type:      CmdCreateCluster,
			ClusterID: parts[1],
			NodeID:    parts[2],
			Host:      parts[3],
			Port:      port,
		}, nil
	}

	return Command{}, fmt.Errorf("unknown command")
}