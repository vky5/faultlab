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

	nodeHost := strings.TrimSpace(opts.NodeHost)
	if nodeHost == "" {
		nodeHost = strings.TrimSpace(e.Cluster.NodeHost)
	}
	if nodeHost == "" {
		nodeHost = "localhost"
	}

	nodeBasePort := opts.NodeBasePort
	if nodeBasePort <= 0 {
		nodeBasePort = e.Cluster.NodeBasePort
	}
	if nodeBasePort <= 0 {
		nodeBasePort = 7001
	}

	nodePortStride := opts.NodePortStride
	if nodePortStride <= 0 {
		nodePortStride = e.Cluster.NodePortStride
	}
	if nodePortStride <= 0 {
		nodePortStride = 1
	}

	batches := make([]CommandBatch, 0, len(e.Timeline))
	for _, step := range e.Timeline {
		commands, err := step.compileCommands(clusterID, controlPlaneHost, controlPlanePort, nodeHost, nodeBasePort, nodePortStride, e.Cluster)
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

func (s TimelineStep) compileCommands(clusterID, cpHost string, cpPort int, nodeHost string, nodeBasePort int, nodePortStride int, clusterCfg ClusterConfig) ([]string, error) {
	switch strings.TrimSpace(s.Action) {
	case "start_cluster":
		return compileStartClusterCommands(clusterID, cpHost, cpPort, nodeHost, nodeBasePort, nodePortStride, clusterCfg), nil
	case "partition":
		return compilePartitionCommands(clusterID, s.Groups, true, clusterCfg)
	case "heal_partition":
		return compilePartitionCommands(clusterID, s.Groups, false, clusterCfg)
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

func compileStartClusterCommands(clusterID, cpHost string, cpPort int, nodeHost string, nodeBasePort int, nodePortStride int, clusterCfg ClusterConfig) []string {
	nodeCount := clusterCfg.Nodes
	if len(clusterCfg.Members) > 0 {
		nodeCount = len(clusterCfg.Members)
	}

	commands := make([]string, 0, nodeCount+1)
	commands = append(commands, fmt.Sprintf("cp new-cluster %s", clusterID))

	for i := 1; i <= nodeCount; i++ {
		nodeID, port, host, err := resolveMember(i, clusterCfg, nodeHost, nodeBasePort, nodePortStride)
		if err != nil {
			continue
		}
		peers := buildPeerList(i, nodeCount, clusterCfg, nodeBasePort, nodePortStride)
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

func compilePartitionCommands(clusterID string, groups [][]int, enabled bool, clusterCfg ClusterConfig) ([]string, error) {
	if len(groups) < 2 {
		return nil, fmt.Errorf("partition actions require at least two groups")
	}

	commands := make([]string, 0)
	for leftIdx := 0; leftIdx < len(groups); leftIdx++ {
		for rightIdx := leftIdx + 1; rightIdx < len(groups); rightIdx++ {
			for _, leftNode := range groups[leftIdx] {
				for _, rightNode := range groups[rightIdx] {
					leftNodeID, err := resolveNodeID(leftNode, clusterCfg)
					if err != nil {
						return nil, err
					}
					rightNodeID, err := resolveNodeID(rightNode, clusterCfg)
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

func resolveTargetGroupLeader(target string, groups [][]int, clusterCfg ClusterConfig) (string, error) {
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
	return resolveNodeID(group[0], clusterCfg)
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

func buildPeerList(nodeIndex, nodeCount int, clusterCfg ClusterConfig, nodeBasePort, nodePortStride int) string {
	peers := make([]string, 0, nodeCount-1)
	for i := 1; i <= nodeCount; i++ {
		if i == nodeIndex {
			continue
		}
		nodeID, port, _, err := resolveMember(i, clusterCfg, "", nodeBasePort, nodePortStride)
		if err != nil {
			continue
		}
		peers = append(peers, fmt.Sprintf("%s:%d", nodeID, port))
	}
	return strings.Join(peers, ",")
}

func resolveNodeID(nodeIndex int, clusterCfg ClusterConfig) (string, error) {
	if nodeIndex <= 0 {
		return "", fmt.Errorf("node index must be greater than zero")
	}

	if len(clusterCfg.Members) > 0 {
		if nodeIndex > len(clusterCfg.Members) {
			return "", fmt.Errorf("node index %d out of range for cluster.members", nodeIndex)
		}
		return clusterCfg.Members[nodeIndex-1].ID, nil
	}

	return fmt.Sprintf("node%d", nodeIndex), nil
}

func resolveMember(nodeIndex int, clusterCfg ClusterConfig, fallbackHost string, nodeBasePort int, nodePortStride int) (string, int, string, error) {
	if nodeIndex <= 0 {
		return "", 0, "", fmt.Errorf("node index must be greater than zero")
	}

	if len(clusterCfg.Members) > 0 {
		if nodeIndex > len(clusterCfg.Members) {
			return "", 0, "", fmt.Errorf("node index %d out of range for cluster.members", nodeIndex)
		}
		member := clusterCfg.Members[nodeIndex-1]
		host := strings.TrimSpace(member.Host)
		if host == "" {
			host = fallbackHost
		}
		if host == "" {
			host = "localhost"
		}
		return member.ID, member.Port, host, nil
	}

	host := fallbackHost
	if strings.TrimSpace(host) == "" {
		host = "localhost"
	}
	return fmt.Sprintf("node%d", nodeIndex), nodeBasePort + ((nodeIndex - 1) * nodePortStride), host, nil
}
