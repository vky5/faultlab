# Vector Clocks In Gossip

## What It Is

A vector clock is a per-key map of `nodeID -> counter`.

Example:

```json
{"node1": 3, "node2": 1}
```

This means the value has seen 3 updates from `node1` and 1 update from `node2`.

## Where It Lives

The value and digest both carry the clock:

```go
type Value struct {
    Data        string
    Version     int64
    Origin      string
    Timestamp   int64
    VectorClock map[string]int64
}

type DigestEntry struct {
    Version     int64
    Origin      string
    Timestamp   int64
    VectorClock map[string]int64
}
```

Reference:
- `internal/node/protocol/gossip/types.go:10`
- `internal/node/protocol/gossip/types.go:26`

The full value stores the data. The digest stores only metadata needed for comparison.

## How It Is Updated

In `Put`, the node copies the current clock and increments only its own entry:

```go
current, ok := g.store[key]

vc := make(map[string]int64)
if ok && current.VectorClock != nil {
    for k, v := range current.VectorClock {
        vc[k] = v
    }
}
vc[g.nodeID]++

g.store[key] = Value{
    Data:        data,
    Version:     version,
    Origin:      g.nodeID,
    Timestamp:   now,
    VectorClock: vc,
}
```

Reference:
- `internal/node/protocol/gossip/gossip.go:401`
- `internal/node/protocol/gossip/gossip.go:412`
- `internal/node/protocol/gossip/gossip.go:419`

So if `node1` writes key `x` twice, its clock may go from `{"node1": 1}` to `{"node1": 2}`.

## How Comparison Works

Vector clocks are compared before timestamps:

```go
func vectorClockCompare(vc1, vc2 map[string]int64) int {
    vc1Greater := false
    vc2Greater := false

    for node := range allNodes {
        v1 := vc1[node]
        v2 := vc2[node]
        if v1 > v2 {
            vc1Greater = true
        } else if v1 < v2 {
            vc2Greater = true
        }
    }

    if vc1Greater && !vc2Greater {
        return 1
    }
    if vc2Greater && !vc1Greater {
        return -1
    }
    return 0
}
```

Reference:
- `internal/node/protocol/gossip/gossip.go:462`

Rule:

- if one clock is greater in at least one component and not smaller in any component, it is newer
- if neither clock dominates, the writes are concurrent

## Why Vector Clocks Work

They work because every node only increments its own counter when it creates a new write. So the clock becomes a compact summary of "which writes this value has seen so far."

Small example:

1. `node1` writes `x=A`
   clock becomes `{"node1": 1}`
2. `node2` receives that value
3. `node2` writes `x=B`
   it starts from the received clock and increments its own entry
   clock becomes `{"node1": 1, "node2": 1}`

Now compare them:

- `{"node1": 1, "node2": 1}` has everything `{"node1": 1}` had
- and it has one extra event from `node2`

So the second value must have happened after the first one. That is why vector clocks capture causality.

If two nodes write independently during a partition, you get something like:

- node1: `{"node1": 2}`
- node2: `{"node2": 1}`

Neither dominates the other, so the system knows these writes are concurrent, not causally ordered.

Another way to say it:

- if value B was created after seeing value A, then B must include all of A's counters
- and B will usually have at least one counter strictly larger

That is why "dominates in every component and is larger in at least one" means "B happened after A."

## What Breaks Ties

If vector clocks do not determine an order, the code falls back to:

1. `Timestamp`
2. `Origin`
3. `Version`

That logic is here:

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
- `internal/node/protocol/gossip/gossip.go:493`

So vector clocks decide causal order. Timestamp-based LWW is only for concurrent writes.

## How A Node Asks For The Newer Value

This protocol does not send "please give me key x" directly.

It does this:

1. node A sends `DIGEST`
2. node B compares A's digest with its local values
3. if B is ahead, B sends `STATE` back immediately
4. if B is behind, B sends its own `DIGEST` back
5. that second digest makes A notice B is behind, so A sends `STATE`

The "I am behind" check is:

```go
for key, remoteEntry := range msg.Digest {
    if localVal, ok := g.store[key]; !ok || compareValueToDigest(localVal, remoteEntry) < 0 {
        behind = true
        break
    }
}
```

If `behind` is true, the node sends its own digest back:

```go
digestMsg := GossipMessage{
    Type:   MsgDigest,
    Digest: make(map[string]DigestEntry),
}
```

Reference:
- `internal/node/protocol/gossip/gossip.go:225`
- `internal/node/protocol/gossip/gossip.go:234`

## Example

Suppose for the same key:

- node1 has `{"node1": 1}`
- node2 has `{"node1": 1, "node2": 1}`

Comparison:

- for `node1`: equal
- for `node2`: node2 is greater

So node2's value dominates node1's value. No timestamp tie-break is needed.

If node2 sends its digest to node1, node1 sees it is behind and sends its own digest back. Then node2 responds with `STATE`, which carries the actual newer value.

If instead the clocks are:

- node1: `{"node1": 1}`
- node2: `{"node2": 1}`

then neither dominates. These writes are concurrent, so the code falls back to:

1. `Timestamp`
2. `Origin`
3. `Version`

## Merge Behavior

When a newer state is accepted, the node merges clocks by taking the max counter for each node:

```go
mergedVC := make(map[string]int64)
for node, clock := range localVal.VectorClock {
    mergedVC[node] = clock
}
for node, clock := range v.VectorClock {
    if clock > mergedVC[node] {
        mergedVC[node] = clock
    }
}

storedValue := v
storedValue.VectorClock = mergedVC
g.store[k] = storedValue
```

Reference:
- `internal/node/protocol/gossip/gossip.go:317`
- `internal/node/protocol/gossip/gossip.go:332`
