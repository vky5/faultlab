package gossip

import "github.com/vky5/faultlab/internal/node/protocol"

// handleState merges incoming values into local state using conflict resolution.
func (g *GossipProtocol) handleState(msg GossipMessage) []protocol.Envelope {
	updated := 0
	for k, v := range msg.State {
		localVal, exists := g.store[k]
		if !exists || isIncomingNewer(localVal, v) {
			eventType := TimelineEventGossipReceive
			if exists {
				eventType = TimelineEventResolve
				g.logGossipf("RESOLVING conflict for key %s: local=(%s, v=%d, o=%s, ts=%d) incoming=(%s, v=%d, o=%s, ts=%d)",
					k, localVal.Data, localVal.Version, localVal.Origin, localVal.Timestamp,
					v.Data, v.Version, v.Origin, v.Timestamp)
			}

			mergedVC := make(map[string]int64)
			if localVal.VectorClock != nil {
				for node, clock := range localVal.VectorClock {
					mergedVC[node] = clock
				}
			}
			if v.VectorClock != nil {
				for node, clock := range v.VectorClock {
					if clock > mergedVC[node] {
						mergedVC[node] = clock
					}
				}
			}

			storedValue := v
			storedValue.VectorClock = mergedVC
			g.store[k] = storedValue
			updated++
			g.emitTimelineEvent(eventType, k, v.Data, v.Version, v.Origin, msg.State[k].Origin, v.Timestamp)
		} else {
			g.logGossipf("REJECTING incoming value for key %s: incoming is older or equal", k)
		}
	}
	g.logGossipf("applied STATE update: %d/%d keys changed", updated, len(msg.State))
	return nil
}


// First step in Conflict resolution. Compare vector clock. If {n1: 1} vs {n1: 2}, the second is newer. If {n1: 1} vs {n1: 1, n2: 1}, the second is newer. If {n1: 2, n2: 1} vs {n1: 1, n2: 2}, they are concurrent.
// vectorClockCompare returns 1 if vc1 > vc2, -1 if vc1 < vc2, 0 if incomparable/equal.
func vectorClockCompare(vc1, vc2 map[string]int64) int {
	vc1Greater := false
	vc2Greater := false

	allNodes := make(map[string]bool)
	for k := range vc1 {
		allNodes[k] = true
	}
	for k := range vc2 {
		allNodes[k] = true
	}

	if len(allNodes) == 0 {
		return 0
	}

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


// isIncomingNewer determines if the incoming value should overwrite the local value based on vector clock, timestamp, origin, and version.
func isIncomingNewer(local, incoming Value) bool {
	vcCmp := vectorClockCompare(local.VectorClock, incoming.VectorClock)
	if vcCmp > 0 {
		return false
	}
	if vcCmp < 0 {
		return true
	}

	// Tie-break using timestamp, then origin, then version
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
}
