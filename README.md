# FaultLab: Distributed Systems Observability Suite

FaultLab is a professional-grade distributed systems fault-injection laboratory. It features a centralized Control Plane, custom database nodes utilizing epidemic gossip protocols, an automated Chaos Hypothesis engine, and a rich Observability UI for post-mortem analysis.

## Key Capabilities
- **Deterministic Testing**: Write `hypothesis.yaml` files to script exact timelines of partitions, crashes, and writes.
- **Post-Mortem Observability**: Visualizes causal logs, state convergence heatmaps, and full-mesh cluster topologies.
- **Chaos Engine**: Directly manipulate network layers to simulate split-brain scenarios and message delays.
- **Conflict Resolution**: Tests CRDTs and Last-Write-Wins (LWW) convergence dynamically.

## Documentation
Please refer to our detailed documentation in the `docs/` folder for architecture and usage guidelines:
- [Architecture Overview](docs/architecture.md)
- [Getting Started / Quick Start](docs/getting-started.md)
- [Hypothesis Engine Guide](docs/hypothesis-engine.md)

## Quick Start

1. **Start the Cluster** (Control plane + 10 nodes backgrounded):
```bash
make cluster
```

2. **Start the Observability UI**:
```bash
make fe
```
*Access the dashboard at http://localhost:3000*

3. **Run a Chaos Experiment**:
Open a new terminal and run a predefined split-brain test:
```bash
make cpcli CMD="run-hypothesis hypotheses/h6_scalability_convergence.yaml"
```

4. **Stop Everything**:
```bash
make stop
```

For more advanced commands, like manually crashing a node or injecting a delay, see the [Getting Started guide](docs/getting-started.md).
