package experiment

import (
	"fmt"
	"strconv"
	"strings"
)

// Compile translates the declarative experiment into executable command batches.
func (e *Experiment) Compile(opts CompileOptions) ([]CommandBatch, error) {
	if err := e.Validate(); err != nil {
		return nil, err
	}

	clusterID := strings.TrimSpace(opts.ClusterID)
	if clusterID == "" {
		clusterID = strings.TrimSpace(e.Cluster.ID)
	}
	if clusterID == "" {
		clusterID = e.Name
	}

	controlPlaneHost := strings.TrimSpace(opts.ControlPlaneHost)
	if controlPlaneHost == "" {
		controlPlaneHost = "localhost"
	}

	controlPlanePort := opts.ControlPlanePort
	if controlPlanePort <= 0 {
		controlPlanePort = 9091
	}

	batches := make([]CommandBatch, 0, len(e.Timeline))
	for _, step := range e.Timeline {
		commands, err := step.compileCommands(clusterID, controlPlaneHost, controlPlanePort, e.Cluster)
		if err != nil {
			return nil, err
		}
		batches = append(batches, CommandBatch{
			At:       step.At.Duration(),
			Commands: commands,
		})
	}

	return batches, nil
}

func (s TimelineStep) compileCommands(clusterID, cpHost string, cpPort int, clusterCfg ClusterConfig) ([]string, error) {
	switch strings.TrimSpace(s.Action) {
	case "start_cluster":
		return compileStartClusterCommands(clusterID, cpHost, cpPort, clusterCfg), nil
	case "partition":
		cmds, err := compilePartitionCommands(clusterID, s.Groups, true, clusterCfg)
		if err != nil { return nil, err }
		return append([]string{fmt.Sprintf("log-lifecycle %s SYSTEM PARTITION \"Splitting cluster into %d partitions\"", clusterID, len(s.Groups))}, cmds...), nil
	case "heal_partition":
		cmds, err := compilePartitionCommands(clusterID, s.Groups, false, clusterCfg)
		if err != nil { return nil, err }
		return append([]string{fmt.Sprintf("log-lifecycle %s SYSTEM HEAL \"Healing all network partitions\"", clusterID)}, cmds...), nil
	case "write":
		nodeID, err := resolveTargetGroupLeader(s.Target, s.Groups, clusterCfg)
		if err != nil {
			return nil, err
		}
		return []string{fmt.Sprintf("kv-put %s %s %s %s", clusterID, nodeID, s.Key, s.Value)}, nil
	default:
		return nil, fmt.Errorf("unknown action %q", s.Action)
	}
}

func compileStartClusterCommands(clusterID, cpHost string, cpPort int, clusterCfg ClusterConfig) []string {
	commands := make([]string, 0, len(clusterCfg.Members)+2)
	commands = append(commands, fmt.Sprintf("cp new-cluster %s", clusterID))
	commands = append(commands, fmt.Sprintf("log-lifecycle %s SYSTEM NODE_JOIN \"Starting cluster members\"", clusterID))

	for i := 1; i <= len(clusterCfg.Members); i++ {
		nodeID, port, host, err := resolveMember(i, clusterCfg)
		if err != nil {
			continue
		}
		peers := buildPeerList(i, clusterCfg)
		command := fmt.Sprintf(
			"start-node %s %d --cluster-id %s --host %s --peers %s --cp-host %s --cp-port %d",
			nodeID,
			port,
			clusterID,
			host,
			peers,
			cpHost,
			cpPort,
		)
		commands = append(commands, command)
	}

	return commands
}

func compilePartitionCommands(clusterID string, groups [][]string, enabled bool, clusterCfg ClusterConfig) ([]string, error) {
	if len(groups) < 2 {
		return nil, fmt.Errorf("partition actions require at least two groups")
	}

	commands := make([]string, 0)
	for leftIdx := 0; leftIdx < len(groups); leftIdx++ {
		for rightIdx := leftIdx + 1; rightIdx < len(groups); rightIdx++ {
			for _, leftNode := range groups[leftIdx] {
				for _, rightNode := range groups[rightIdx] {
					leftNodeID, err := resolveGroupNodeID(leftNode, clusterCfg)
					if err != nil {
						return nil, err
					}
					rightNodeID, err := resolveGroupNodeID(rightNode, clusterCfg)
					if err != nil {
						return nil, err
					}
					commands = append(commands, fmt.Sprintf(
						"fault-partition %s %s %s %t",
						clusterID,
						leftNodeID,
						rightNodeID,
						enabled,
					))
				}
			}
		}
	}

	return commands, nil
}

func resolveTargetGroupLeader(target string, groups [][]string, clusterCfg ClusterConfig) (string, error) {
	groupIndex, err := parseGroupTarget(target)
	if err != nil {
		return "", err
	}
	if groupIndex < 1 || groupIndex > len(groups) {
		return "", fmt.Errorf("target %q is out of range", target)
	}
	group := groups[groupIndex-1]
	if len(group) == 0 {
		return "", fmt.Errorf("target %q has no nodes", target)
	}
	return resolveGroupNodeID(group[0], clusterCfg)
}

func parseGroupTarget(target string) (int, error) {
	trimmed := strings.ToLower(strings.TrimSpace(target))
	if !strings.HasPrefix(trimmed, "group") {
		return 0, fmt.Errorf("target %q must look like group1, group2, ...", target)
	}
	value := strings.TrimPrefix(trimmed, "group")
	index, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("target %q is invalid", target)
	}
	return index, nil
}

func buildPeerList(nodeIndex int, clusterCfg ClusterConfig) string {
	peers := make([]string, 0, len(clusterCfg.Members)-1)
	for i := 1; i <= len(clusterCfg.Members); i++ {
		if i == nodeIndex {
			continue
		}
		nodeID, port, _, err := resolveMember(i, clusterCfg)
		if err != nil {
			continue
		}
		peers = append(peers, fmt.Sprintf("%s:%d", nodeID, port))
	}
	return strings.Join(peers, ",")
}

func resolveGroupNodeID(nodeID string, clusterCfg ClusterConfig) (string, error) {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return "", fmt.Errorf("node id is required")
	}

	if len(clusterCfg.Members) > 0 {
		for _, member := range clusterCfg.Members {
			if member.ID == nodeID {
				return nodeID, nil
			}
		}
		return "", fmt.Errorf("node id %q not found in cluster.members", nodeID)
	}

	return "", fmt.Errorf("node id %q not found in cluster.members", nodeID)
}

func resolveMember(nodeIndex int, clusterCfg ClusterConfig) (string, int, string, error) {
	if nodeIndex <= 0 {
		return "", 0, "", fmt.Errorf("node index must be greater than zero")
	}

	if nodeIndex > len(clusterCfg.Members) {
		return "", 0, "", fmt.Errorf("node index %d out of range for cluster.members", nodeIndex)
	}

	member := clusterCfg.Members[nodeIndex-1]
	host := strings.TrimSpace(member.Host)
	if strings.TrimSpace(host) == "" {
		host = "localhost"
	}
	return member.ID, member.Port, host, nil
}
