package gossip

import "time"

// Put stores or updates a local value and emits a write timeline event.
func (g *GossipProtocol) Put(key, data string) {
	logicalTS := time.Now().UnixMilli()
	current, ok := g.store[key]

	vc := make(map[string]int64)
	if ok && current.VectorClock != nil {
		for k, v := range current.VectorClock {
			vc[k] = v
		}
	}
	vc[g.nodeID]++

	var version int64 = 1
	if ok {
		version = current.Version + 1
	}

	g.store[key] = Value{
		Data:        data,
		Version:     version,
		Origin:      g.nodeID,
		Timestamp:   logicalTS,
		VectorClock: vc,
	}
	g.logGossipf("PUT created/updated key=%s version=%d vc=%v", key, version, vc)

	v := g.store[key]
	g.emitTimelineEvent(TimelineEventWrite, key, v.Data, v.Version, v.Origin, g.nodeID, v.Timestamp)
}

// Get retrieves a value from the local store.
func (g *GossipProtocol) Get(key string) (string, bool) {
	val, ok := g.store[key]
	if !ok {
		return "", false
	}
	return val.Data, true
}

func (g *GossipProtocol) GetWithMetadata(key string) (string, int64, string, bool) {
	val, ok := g.store[key]
	if !ok {
		return "", 0, "", false
	}
	return val.Data, val.Version, val.Origin, true
}

// Delete removes a key from the local store.
func (g *GossipProtocol) Delete(key string) {
	if _, ok := g.store[key]; ok {
		delete(g.store, key)
		g.logGossipf("DELETE key=%s", key)
	}
}
