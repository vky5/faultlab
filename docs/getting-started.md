# Getting Started with FaultLab

This guide will walk you through spinning up your first 10-node distributed cluster, running a chaos engineering hypothesis, and visualizing the results.

## Prerequisites
- **Go 1.22+**
- **Node.js** (for the UI)
- **Make**

## 1. Start the Cluster
To easily start the Control Plane and 10 individual database nodes locally, use the provided Make target:
```bash
make cluster
```
*This starts the Control Plane on port `9000`, and nodes `node1` through `node10` on ports `7001-7010`.*

## 2. Start the Observability UI
Open a new terminal and start the Next.js frontend:
```bash
make fe
```
Navigate to `http://localhost:3000` in your browser. You will use this to inspect the results of your experiment.

## 3. Run a Hypothesis
FaultLab uses YAML files to define test scenarios. To run the scalability and convergence hypothesis (which splits the 10 nodes in half, writes conflicting data, and then heals the partition):

Open a third terminal and run:
```bash
make cpcli CMD="run-hypothesis hypotheses/h6_scalability_convergence.yaml"
```

## 4. Analyze the Results
Once the CLI reports the experiment is complete, go to the UI at `http://localhost:3000`.
- Watch the **Causal Narrative** to see exactly when the partition occurred and how nodes resolved the conflicts.
- Hover over and click on any **Topology Graph** to expand it and inspect the network mesh.
- Export the **Post-Mortem PDF** for a complete report including the Convergence Waterfall.

## 5. Handling Common Operations

### Killing a Specific Node
If you want to manually test fault tolerance by crashing a single node, you can use the CLI:
```bash
make cpcli CMD="fault-crash c1 node3"
```
To bring it back:
```bash
make cpcli CMD="fault-recover c1 node3"
```

### Manual Writes
To manually write a key to a specific node and watch it propagate:
```bash
make cpcli CMD="kv-put c1 node1 my_key my_value"
```

## 6. Teardown
To cleanly stop all nodes and the control plane:
```bash
make stop
```
