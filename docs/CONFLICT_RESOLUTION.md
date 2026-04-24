# Conflict Resolution In Gossip

## What Counts As A Conflict

A conflict means two replicas have different versions of the same key and the system must decide which one wins.

In this implementation, conflicts are checked when:

- merging incoming `STATE`
- comparing a local value against a remote `DIGEST`

## Decision Order

The code resolves conflicts in this order:

1. `VectorClock`
2. `Timestamp`
3. `Origin`
4. `Version`

The key comparison logic is:

```go
vcCmp := vectorClockCompare(local.VectorClock, incoming.VectorClock)
if vcCmp > 0 {
    return false
}
if vcCmp < 0 {
    return true
}

if incoming.Timestamp > local.Timestamp {
    return true
}
if incoming.Timestamp < local.Timestamp {
    return false
}
if incoming.Origin > local.Origin {
    return true
}
if incoming.Origin < local.Origin {
    return false
}
return incoming.Version > local.Version
```

Reference:
- Conflict decision logic: internal/node/protocol/gossip/conflict_resolution.go
- Digest comparison usage: internal/node/protocol/gossip/propagation.go

## Why This Order

- `VectorClock` decides causal order
- `Timestamp` is only used when the writes are concurrent
- `Origin` gives a deterministic winner if timestamps tie
- `Version` is the last fallback

Note: write timestamps are protocol logical ticks (not wall-clock milliseconds). This removes cross-node clock-skew bias in concurrent conflict decisions.

So this is not pure LWW. It is:

causality first, then LWW-style tie-breaking.

## Where Conflict Resolution Is Used

### 1. During `STATE` Merge

If an incoming value is newer, it replaces the local value:

```go
if localVal, ok := g.store[k]; !ok || isIncomingNewer(localVal, v) {
    storedValue := v
    storedValue.VectorClock = mergedVC
    g.store[k] = storedValue
}
```

Reference:
- internal/node/protocol/gossip/conflict_resolution.go

### 2. During `DIGEST` Comparison

When a node receives a digest, it compares local metadata with remote metadata:

```go
for key, remoteEntry := range msg.Digest {
    if localVal, ok := g.store[key]; !ok || compareValueToDigest(localVal, remoteEntry) < 0 {
        behind = true
        break
    }
}
```

If `behind` becomes `true`, the node sends its own digest back to trigger reverse sync.

Reference:
- internal/node/protocol/gossip/propagation.go

## Example 1: No Tie-Break Needed

- local: `{"n1": 1}`
- remote: `{"n1": 1, "n2": 1}`

Remote dominates local, so remote is newer.

No timestamp or origin check is needed.

## Example 2: Tie-Break Needed

- local: `{"n1": 1}`
- remote: `{"n2": 1}`

Neither dominates the other. These writes are concurrent.

So the code falls back to:

1. higher `Timestamp`
2. higher `Origin`
3. higher `Version`

## How The Newer Value Gets Sent

This protocol does not send an explicit "give me key x" request.

It works like this:

1. node A sends `DIGEST`
2. node B compares
3. if B has the newer value, B sends `STATE`
4. if B is behind, B sends its own `DIGEST` back
5. that causes A to send `STATE`

So conflict resolution chooses the winner, and the digest/state exchange moves that winner across replicas.

## File Separation

Gossip internals are now split by responsibility:

- internal/node/protocol/gossip/propagation.go
    - digest and state exchange flow
    - peer selection and sender parsing
    - digest-side metadata comparison for sync direction
- internal/node/protocol/gossip/conflict_resolution.go
    - state merge behavior
    - vector clock comparison and causal ordering
    - LWW tie-break fallback rules
- internal/node/protocol/gossip/crud.go
    - local Put/Get/Delete APIs
    - vector clock increment on local write
