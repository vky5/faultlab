# FaultLab Architecture

FaultLab is a comprehensive distributed systems testing laboratory built to observe, validate, and analyze the behavior of distributed consensus and replication protocols (like Gossip and CRDTs) under simulated network failures.

## Core Components

### 1. Control Plane
The central orchestrator responsible for managing the cluster topology and injecting faults.
- **HTTP/gRPC API**: Exposes endpoints to control the cluster.
- **Hypothesis Engine**: Parses YAML files defining experiments, manages the execution timeline, and evaluates assertions upon completion.
- **Ledger/Metrics**: Aggregates all causal events (writes, resolves, partitions) into a unified timeline.

### 2. Database Nodes
The individual replicas participating in the distributed system.
- **Gossip Protocol**: Nodes communicate via epidemic broadcast (gossip) to share state.
- **Conflict Resolution**: Implements LWW (Last-Write-Wins) using logical clocks and timestamps to resolve split-brain divergences.
- **Fault Injection Engine**: Runs locally on each node, capable of dropping packets, delaying messages, or completely partitioning network access based on Control Plane commands.

### 3. Observability UI (Frontend)
A Next.js application that visualizes the state of the cluster.
- **Post-Mortem Investigation**: Transforms raw logs into a causal execution narrative.
- **Dynamic Topology**: Renders full-mesh cluster graphs and highlights partitioned networks in real-time.
- **State Heatmaps**: Tracks the propagation of values across the cluster over time.

## Execution Flow
1. **Definition**: An operator defines a cluster topology and timeline in a `hypothesis.yaml` file.
2. **Initialization**: The Control Plane initializes the nodes, registers them, and begins the clock.
3. **Fault Injection**: At specified timestamps, the Control Plane instructs specific nodes to sever their connections to peers.
4. **Operations**: Client writes are sent to isolated partitions, forcing state divergence.
5. **Resolution**: The partition is healed, and nodes gossip to converge on a single state.
6. **Analysis**: The execution ledger is analyzed by the frontend to verify convergence times and correctness.
