# Hypothesis Engine

The Hypothesis Engine is the core of FaultLab's deterministic testing suite. It allows operators to define distributed system experiments as code.

## YAML Structure

A standard hypothesis file contains the following blocks:

### 1. Metadata
```yaml
name: H6_scalability_convergence
hypothesis: "Gossip convergence scales logarithmically."
description: "Tests a massive 10-node split-brain scenario."
```

### 2. Cluster Definition
Defines the topology required for the test.
```yaml
cluster:
  id: h6_cluster_large
  members:
    - id: node1
      port: 7601
    - id: node2
      port: 7602
    ...
```

### 3. Timeline (The Execution Ledger)
The `timeline` strictly dictates when actions occur during the simulation.
```yaml
timeline:
  - at: 0s
    action: start_cluster

  - at: 4s
    action: partition
    groups:
      - [node1, node2, node3, node4, node5]
      - [node6, node7, node8, node9, node10]

  - at: 8s
    action: write
    key: scale_test
    value: right_side_write
    target: group2

  - at: 15s
    action: heal_partition
```
Actions supported:
- `start_cluster`: Bootstraps the nodes.
- `partition`: Splits the network into defined groups.
- `heal_partition`: Restores full network mesh connectivity.
- `write`: Injects a KV pair to a node or group.
- `crash`: Simulates a node process dying.
- `recover`: Simulates a node process restarting.

### 4. Validation
Tells the engine what to verify at the end of the timeline.
```yaml
validation:
  check_convergence: true
  check_keys:
    - scale_test
```
If convergence fails or takes too long, the CLI will report a failure, and the Observability UI will highlight the divergence in red.
