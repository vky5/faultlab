# Fault Engine

The fault package provides an in-memory fault injection engine used by node runtime/session components.

## Implemented Fault Types

1. Crash fault
- API: `Crash()`, `Recover()`, `IsCrashed()`
- Behavior: Marks node as crashed from the fault layer perspective.

2. Probabilistic packet drop
- API: `SetDropRate(p float64)`, `ShouldDrop()`
- Behavior: Drops an operation with probability `p` where `0.0 <= p <= 1.0`.

3. Artificial delay
- API: `SetDelay(ms int)`, `ApplyDelay()`
- Behavior: Adds fixed latency in milliseconds before proceeding.

4. Network partition (directional)
- API: `Partition(peer string)`, `Heal(peer string)`, `IsPartitioned(peer string)`
- Behavior: Blocks communication to specific peers.
- Important: Partition is directional (A partitioned from B does not imply B partitioned from A).

## Engine Characteristics

- Thread-safe via internal `sync.RWMutex`
- State is process-local and non-persistent
- Partition map is per-peer: `map[string]bool`

## Integration Points

- Runtime accepts engine at startup (`Start(fe *fault.Engine)`).
- Node session accepts a `FaultProvider` abstraction with:
	- `IsCrashed()`
	- `ShouldDrop()`
	- `ApplyDelay()`
	- `IsPartitioned(peer string)`

## Current Scope

- Focused on transport-level fault simulation.
- Does not yet include clock skew, message duplication, message reordering, or Byzantine faults.
