# Subsystem Difficulty

This project is easier to understand if you break it into a few subsystems. The scores below are relative to this codebase, where 1 is easiest and 5 is hardest.

## 1. Experiment Parsing and Compilation - 2/5

What it does:
- Loads YAML experiment files.
- Validates the scenario shape.
- Compiles timeline steps into concrete control-plane commands.

Why it is moderately easy:
- The logic is mostly deterministic string/struct transformation.
- The main complexity is keeping YAML shape, validation, and command generation aligned.

Key files:
- `internal/experiment/experiment.go`
- `internal/experiment/command_parser.go`

## 2. Control Plane Command Handling - 3/5

What it does:
- Receives commands from cpcli and startup flows.
- Parses commands.
- Routes them to the actor loop.

Why it is medium difficulty:
- There are several entrypoints.
- It has to coordinate process lifecycle, startup bootstrap, and runtime actions.
- The code is straightforward, but the wiring can be easy to misread.

Key files:
- `internal/controlplane/parser.go`
- `internal/controlplane/runtime_commands.go`
- `internal/controlplane/actor.go`
- `cmd/controlplane/main.go`
- `cmd/cpcli/main.go`

## 3. Cluster State and Manager Layer - 3/5

What it does:
- Stores clusters and nodes.
- Registers/removes nodes.
- Tracks faults, protocol state, and node capabilities.

Why it is medium difficulty:
- The data model is simple.
- The hard part is making sure state changes stay consistent across command, service, and node interactions.

Key files:
- `internal/cluster/config.go`
- `internal/cluster/manager/*.go`
- `internal/controlplane/service/service.go`

## 4. Node Runtime - 4/5

What it does:
- Runs the node event loop.
- Handles ticks, messages, protocol actions, and protocol swaps.
- Bridges protocol behavior to real network and local state changes.

Why it is hard:
- It is concurrency-heavy.
- It needs deterministic ordering.
- It sits between control-plane requests and protocol logic.

Key files:
- `internal/node/runtime/*.go`
- `internal/node/session/*.go`
- `internal/node/server.go`

## 5. Protocol Implementations - 4/5

What it does:
- Implements gossip and baseline membership behavior.
- Encodes and decodes envelopes.
- Applies versioning, state exchange, and peer discovery.

Why it is hard:
- Protocol bugs are subtle.
- Small logic mistakes can create divergence, replay-like behavior, or non-convergence.
- Correctness depends on message ordering, version rules, and merge behavior.

Key files:
- `internal/node/protocol/gossip/*.go`
- `internal/node/protocol/baseline/*.go`
- `internal/node/protocol/*.go`

## 6. Fault Injection Layer - 5/5

What it does:
- Decides whether traffic is dropped, delayed, or blocked.
- Applies partitions and crash behavior.
- Shapes node behavior under failure.

Why it is the hardest:
- It interacts with every other subsystem.
- Bugs often appear only under specific timing or topology conditions.
- It is easy to accidentally make the system look broken when the fault model is the real source of behavior.

Key files:
- `internal/fault/*.go`
- `internal/node/exec/*.go`
- `internal/node/session/node.go`

## 7. Experiment Execution Flow - 4/5

What it does:
- Ties YAML experiments, control-plane commands, and node behavior together.
- Runs scenarios over time.
- Launches node processes and bootstrap actions.

Why it is hard:
- It is not one subsystem; it is the orchestration layer connecting all of them.
- A mistake here can make a valid experiment look invalid, or vice versa.

Key files:
- `internal/engine/controlplane.go`
- `internal/controlplane/actor.go`
- `cmd/controlplane/main.go`

## Summary

Hardest parts:
- Fault injection
- Node runtime
- Protocol behavior

Easiest parts:
- YAML parsing
- Simple command formatting
- Pure data validation

If you are trying to understand the project quickly, start with:
1. `internal/experiment/experiment.go`
2. `internal/experiment/command_parser.go`
3. `cmd/controlplane/main.go`
4. `internal/controlplane/actor.go`
5. `internal/node/runtime/*.go`
