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
	case "help":
		return Command{Type: CmdHelp}, nil

	case "new-cluster":
		if len(parts) < 2 {
			return Command{}, fmt.Errorf("usage: new-cluster <cluster-id> [protocol]")
		}
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
		if len(parts) < 3 {
			return Command{}, fmt.Errorf("usage: remove-node <cluster-id> <node-id>")
		}
		return Command{
			Type:      CmdRemoveNode,
			ClusterID: parts[1],
			NodeID:    parts[2],
		}, nil

	case "list-nodes":
		if len(parts) < 2 {
			return Command{}, fmt.Errorf("usage: list-nodes <cluster-id>")
		}
		return Command{
			Type:      CmdListNodes,
			ClusterID: parts[1],
		}, nil

	case "list-clusters":
		return Command{Type: CmdListClusters}, nil

	case "add-node":
		if len(parts) < 5 {
			return Command{}, fmt.Errorf("usage: add-node <cluster-id> <node-id> <host> <port>")
		}

		port, err := strconv.Atoi(parts[4])
		if err != nil {
			return Command{}, fmt.Errorf("invalid port: %v", err)
		}

		return Command{
			Type:      CmdAddNode,
			ClusterID: parts[1],
			NodeID:    parts[2],
			Host:      parts[3],
			Port:      port,
		}, nil

	case "set-fault":
		// usage:
		// set-fault <cluster-id> <node-id> <crashed:true|false> <drop-rate:0..1> <delay-ms:int> [partition-csv]
		if len(parts) < 6 {
			return Command{}, fmt.Errorf("usage: set-fault <cluster-id> <node-id> <crashed:true|false> <drop-rate:0..1> <delay-ms:int> [partition-csv]")
		}

		crashed, err := strconv.ParseBool(parts[3])
		if err != nil {
			return Command{}, fmt.Errorf("invalid crashed flag: %v", err)
		}

		dropRate, err := strconv.ParseFloat(parts[4], 64)
		if err != nil {
			return Command{}, fmt.Errorf("invalid drop-rate: %v", err)
		}

		delayMs, err := strconv.Atoi(parts[5])
		if err != nil {
			return Command{}, fmt.Errorf("invalid delay-ms: %v", err)
		}

		var partition []string
		if len(parts) >= 7 && strings.TrimSpace(parts[6]) != "" {
			partition = strings.Split(parts[6], ",")
		}

		return Command{
			Type:      CmdSetFaultParams,
			ClusterID: parts[1],
			NodeID:    parts[2],
			Crashed:   crashed,
			DropRate:  dropRate,
			DelayMs:   delayMs,
			Partition: partition,
		}, nil

	case "fault-crash":
		if len(parts) < 3 {
			return Command{}, fmt.Errorf("usage: fault-crash <cluster-id> <node-id>")
		}
		return Command{
			Type:      CmdCrashNode,
			ClusterID: parts[1],
			NodeID:    parts[2],
		}, nil

	case "fault-recover":
		if len(parts) < 3 {
			return Command{}, fmt.Errorf("usage: fault-recover <cluster-id> <node-id>")
		}
		return Command{
			Type:      CmdRecoverNode,
			ClusterID: parts[1],
			NodeID:    parts[2],
		}, nil

	case "fault-drop":
		if len(parts) < 4 {
			return Command{}, fmt.Errorf("usage: fault-drop <cluster-id> <node-id> <drop-rate:0..1>")
		}
		dropRate, err := strconv.ParseFloat(parts[3], 64)
		if err != nil {
			return Command{}, fmt.Errorf("invalid drop-rate: %v", err)
		}
		return Command{
			Type:      CmdSetDropRate,
			ClusterID: parts[1],
			NodeID:    parts[2],
			DropRate:  dropRate,
		}, nil

	case "fault-delay":
		if len(parts) < 4 {
			return Command{}, fmt.Errorf("usage: fault-delay <cluster-id> <node-id> <delay-ms:int>")
		}
		delayMs, err := strconv.Atoi(parts[3])
		if err != nil {
			return Command{}, fmt.Errorf("invalid delay-ms: %v", err)
		}
		return Command{
			Type:      CmdSetDelay,
			ClusterID: parts[1],
			NodeID:    parts[2],
			DelayMs:   delayMs,
		}, nil

	case "fault-partition":
		if len(parts) < 5 {
			return Command{}, fmt.Errorf("usage: fault-partition <cluster-id> <node-id> <peer-id> <enabled:true|false>")
		}
		enabled, err := strconv.ParseBool(parts[4])
		if err != nil {
			return Command{}, fmt.Errorf("invalid enabled flag: %v", err)
		}
		return Command{
			Type:      CmdSetPartition,
			ClusterID: parts[1],
			NodeID:    parts[2],
			PeerID:    parts[3],
			Enabled:   enabled,
		}, nil
	}

	return Command{}, fmt.Errorf("unknown command")
}
