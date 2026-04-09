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
		cmd := NewCommand(CmdHelp)
		return cmd, nil

	case "start-node":
		if len(parts) < 3 {
			return Command{}, fmt.Errorf("usage: start-node <node-id> <port> [--cluster-id <id>] [--host <host>] [--peers <csv>] [--cp-host <host>] [--cp-port <port>]")
		}

		port, err := strconv.Atoi(parts[2])
		if err != nil {
			return Command{}, fmt.Errorf("invalid port: %v", err)
		}

		cmd := NewCommand(CmdStartNodeProcess)
		cmd.NodeID = parts[1]
		cmd.Port = port
		cmd.ClusterID = "default"
		cmd.Host = "localhost"

		for i := 3; i < len(parts); i++ {
			arg := parts[i]
			switch arg {
			case "--cluster-id":
				if i+1 >= len(parts) {
					return Command{}, fmt.Errorf("missing value for --cluster-id")
				}
				cmd.ClusterID = parts[i+1]
				i++
			case "--host":
				if i+1 >= len(parts) {
					return Command{}, fmt.Errorf("missing value for --host")
				}
				cmd.Host = parts[i+1]
				i++
			case "--peers":
				if i+1 >= len(parts) {
					return Command{}, fmt.Errorf("missing value for --peers")
				}
				cmd.PeersCSV = parts[i+1]
				i++
			case "--cp-host":
				if i+1 >= len(parts) {
					return Command{}, fmt.Errorf("missing value for --cp-host")
				}
				cmd.CPHost = parts[i+1]
				i++
			case "--cp-port":
				if i+1 >= len(parts) {
					return Command{}, fmt.Errorf("missing value for --cp-port")
				}
				cpPort, err := strconv.Atoi(parts[i+1])
				if err != nil {
					return Command{}, fmt.Errorf("invalid --cp-port: %v", err)
				}
				cmd.CPPort = cpPort
				i++
			default:
				return Command{}, fmt.Errorf("unknown option for start-node: %s", arg)
			}
		}

		return cmd, nil

	case "stop-node":
		if len(parts) < 2 {
			return Command{}, fmt.Errorf("usage: stop-node <node-id>")
		}
		cmd := NewCommand(CmdStopNodeProcess)
		cmd.NodeID = parts[1]
		return cmd, nil

	case "list-node-procs":
		cmd := NewCommand(CmdListNodeProcesses)
		return cmd, nil

	case "new-cluster":
		if len(parts) < 2 {
			return Command{}, fmt.Errorf("usage: new-cluster <cluster-id> [--protocol <gossip|raft>]")
		}

		protocol := "gossip"
		for i := 2; i < len(parts); i++ {
			arg := parts[i]

			if arg == "--protocol" || arg == "-protocol" || arg == "-p" {
				if i+1 >= len(parts) {
					return Command{}, fmt.Errorf("usage: new-cluster <cluster-id> [--protocol <gossip|raft>]")
				}
				protocol = parts[i+1]
				i++
				continue
			}

			// Backward compatibility with old positional syntax: new-cluster <id> <protocol>
			if i == 2 && !strings.HasPrefix(arg, "-") {
				protocol = arg
				continue
			}

			return Command{}, fmt.Errorf("unknown option for new-cluster: %s", arg)
		}

		cmd := NewCommand(CmdCreateCluster)
		cmd.ClusterID = parts[1]
		cmd.Protocol = protocol
		return cmd, nil

	case "remove-node":
		if len(parts) < 3 {
			return Command{}, fmt.Errorf("usage: remove-node <cluster-id> <node-id>")
		}
		cmd := NewCommand(CmdRemoveNode)
		cmd.ClusterID = parts[1]
		cmd.NodeID = parts[2]
		return cmd, nil

	case "list-nodes":
		if len(parts) < 2 {
			return Command{}, fmt.Errorf("usage: list-nodes <cluster-id>")
		}
		cmd := NewCommand(CmdListNodes)
		cmd.ClusterID = parts[1]
		return cmd, nil

	case "list-clusters":
		cmd := NewCommand(CmdListClusters)
		return cmd, nil

	case "add-node":
		if len(parts) < 5 {
			return Command{}, fmt.Errorf("usage: add-node <cluster-id> <node-id> <host> <port>")
		}

		port, err := strconv.Atoi(parts[4])
		if err != nil {
			return Command{}, fmt.Errorf("invalid port: %v", err)
		}

		cmd := NewCommand(CmdAddNode)
		cmd.ClusterID = parts[1]
		cmd.NodeID = parts[2]
		cmd.Host = parts[3]
		cmd.Port = port
		return cmd, nil

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

		cmd := NewCommand(CmdSetFaultParams)
		cmd.ClusterID = parts[1]
		cmd.NodeID = parts[2]
		cmd.Crashed = crashed
		cmd.DropRate = dropRate
		cmd.DelayMs = delayMs
		cmd.Partition = partition
		return cmd, nil

	case "fault-crash":
		if len(parts) < 3 {
			return Command{}, fmt.Errorf("usage: fault-crash <cluster-id> <node-id>")
		}
		cmd := NewCommand(CmdCrashNode)
		cmd.ClusterID = parts[1]
		cmd.NodeID = parts[2]
		return cmd, nil

	case "fault-recover":
		if len(parts) < 3 {
			return Command{}, fmt.Errorf("usage: fault-recover <cluster-id> <node-id>")
		}
		cmd := NewCommand(CmdRecoverNode)
		cmd.ClusterID = parts[1]
		cmd.NodeID = parts[2]
		return cmd, nil

	case "fault-drop":
		if len(parts) < 4 {
			return Command{}, fmt.Errorf("usage: fault-drop <cluster-id> <node-id> <drop-rate:0..1>")
		}
		dropRate, err := strconv.ParseFloat(parts[3], 64)
		if err != nil {
			return Command{}, fmt.Errorf("invalid drop-rate: %v", err)
		}
		cmd := NewCommand(CmdSetDropRate)
		cmd.ClusterID = parts[1]
		cmd.NodeID = parts[2]
		cmd.DropRate = dropRate
		return cmd, nil

	case "fault-delay":
		if len(parts) < 4 {
			return Command{}, fmt.Errorf("usage: fault-delay <cluster-id> <node-id> <delay-ms:int>")
		}
		delayMs, err := strconv.Atoi(parts[3])
		if err != nil {
			return Command{}, fmt.Errorf("invalid delay-ms: %v", err)
		}
		cmd := NewCommand(CmdSetDelay)
		cmd.ClusterID = parts[1]
		cmd.NodeID = parts[2]
		cmd.DelayMs = delayMs
		return cmd, nil

	case "fault-partition":
		if len(parts) < 5 {
			return Command{}, fmt.Errorf("usage: fault-partition <cluster-id> <node-id> <peer-id> <enabled:true|false>")
		}
		enabled, err := strconv.ParseBool(parts[4])
		if err != nil {
			return Command{}, fmt.Errorf("invalid enabled flag: %v", err)
		}
		cmd := NewCommand(CmdSetPartition)
		cmd.ClusterID = parts[1]
		cmd.NodeID = parts[2]
		cmd.PeerID = parts[3]
		cmd.Enabled = enabled
		return cmd, nil

	case "kv-put":
		if len(parts) < 5 {
			return Command{}, fmt.Errorf("usage: kv-put <cluster-id> <node-id> <key> <value>")
		}
		cmd := NewCommand(CmdKVPut)
		cmd.ClusterID = parts[1]
		cmd.NodeID = parts[2]
		cmd.Key = parts[3]
		cmd.Value = parts[4]
		return cmd, nil

	case "kv-get":
		if len(parts) < 4 {
			return Command{}, fmt.Errorf("usage: kv-get <cluster-id> <node-id> <key>")
		}
		cmd := NewCommand(CmdKVGet)
		cmd.ClusterID = parts[1]
		cmd.NodeID = parts[2]
		cmd.Key = parts[3]
		return cmd, nil

	case "set-protocol":
		if len(parts) < 3 {
			return Command{}, fmt.Errorf("usage: set-protocol <cluster-id> <protocol>")
		}
		cmd := NewCommand(CmdSetClusterProtocol)
		cmd.ClusterID = parts[1]
		cmd.Protocol = parts[2]
		return cmd, nil
	}

	return Command{}, fmt.Errorf("unknown command")
}
