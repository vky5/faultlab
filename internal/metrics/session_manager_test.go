package metrics

import (
	"testing"
	"time"
)

func TestSessionManagerLifecycleAndTracking(t *testing.T) {
	mgr := NewSessionManager()
	start := time.Unix(100, 0)

	if err := mgr.Start("c1", start); err != nil {
		t.Fatalf("start session: %v", err)
	}
	if !mgr.IsActive("c1") {
		t.Fatalf("expected active session")
	}

	if err := mgr.TrackKey("c1", "k1"); err != nil {
		t.Fatalf("track key: %v", err)
	}

	snap, err := mgr.GetSession("c1")
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if len(snap.TrackedKeys) != 1 || snap.TrackedKeys[0] != "k1" {
		t.Fatalf("unexpected tracked keys: %#v", snap.TrackedKeys)
	}

	if err := mgr.Stop("c1", start.Add(10*time.Second)); err != nil {
		t.Fatalf("stop session: %v", err)
	}
	if mgr.IsActive("c1") {
		t.Fatalf("expected inactive session")
	}

	if err := mgr.TrackKey("c1", "k2"); err == nil {
		t.Fatalf("expected error tracking key on stopped session")
	}
}

func TestSessionManagerRecordAndCompute(t *testing.T) {
	mgr := NewSessionManager()
	start := time.Unix(200, 0)

	if err := mgr.Start("clusterA", start); err != nil {
		t.Fatalf("start session: %v", err)
	}

	if err := mgr.RecordSnapshot("clusterA", "k", start, map[string]NodeState{
		"nodeA": {Exists: true, Value: "3", Version: 1, Origin: "nodeA", HasMetadata: true},
		"nodeB": {Exists: true, Value: "3", Version: 2, Origin: "nodeB", HasMetadata: true},
		"nodeC": {Exists: true, Value: "3", Version: 1, Origin: "nodeA", HasMetadata: true},
	}); err != nil {
		t.Fatalf("record snapshot 1: %v", err)
	}

	if err := mgr.RecordSnapshot("clusterA", "k", start.Add(5*time.Second), map[string]NodeState{
		"nodeA": {Exists: true, Value: "3", Version: 2, Origin: "nodeB", HasMetadata: true},
		"nodeB": {Exists: true, Value: "3", Version: 2, Origin: "nodeB", HasMetadata: true},
		"nodeC": {Exists: true, Value: "3", Version: 2, Origin: "nodeB", HasMetadata: true},
	}); err != nil {
		t.Fatalf("record snapshot 2: %v", err)
	}

	res, err := mgr.ComputeKeyResult("clusterA", "k")
	if err != nil {
		t.Fatalf("compute key result: %v", err)
	}

	if res.ConvergenceTime == nil || *res.ConvergenceTime != 5*time.Second {
		t.Fatalf("unexpected convergence time: %v", res.ConvergenceTime)
	}
	if got := res.StaleDurationPerNode["nodeB"]; got != 5*time.Second {
		t.Fatalf("unexpected stale duration for nodeB: %v", got)
	}
	if got := res.StaleDurationPerNode["nodeA"]; got != 0 {
		t.Fatalf("unexpected stale duration for nodeA: %v", got)
	}

	all, err := mgr.ComputeAllResults("clusterA")
	if err != nil {
		t.Fatalf("compute all results: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("unexpected result count: %d", len(all))
	}
}

func TestSessionManagerTimeline(t *testing.T) {
	mgr := NewSessionManager()
	start := time.Unix(300, 0)
	mgr.Start("clusterT", start)

	// Simulation: node1 writes A, then node2 receives it
	mgr.RecordTimelineEvent("clusterT", "k1", start.Add(1*time.Second), TimelineEvent{
		NodeID: "node1", Key: "k1", Value: "A", Version: 1, Origin: "node1", EventType: "WRITE",
	})
	mgr.RecordTimelineEvent("clusterT", "k1", start.Add(2*time.Second), TimelineEvent{
		NodeID: "node2", Key: "k1", Value: "A", Version: 1, Origin: "node1", EventType: "GOSSIP_RECEIVE",
	})

	res, _ := mgr.ComputeKeyResult("clusterT", "k1")

	if len(res.Timeline) != 2 {
		t.Fatalf("expected 2 timeline events, got %d", len(res.Timeline))
	}

	// Check convergence curve:
	// t=1: 1 distinct value (node1 has A, node2 has nothing)
	// t=2: 1 distinct value (node1 has A, node2 has A)
	if len(res.ConvergenceCurve) != 2 {
		t.Fatalf("expected convergence curve length 2, got %d", len(res.ConvergenceCurve))
	}
	
	// node1 writes B (conflict)
	mgr.RecordTimelineEvent("clusterT", "k1", start.Add(3*time.Second), TimelineEvent{
		NodeID: "node1", Key: "k1", Value: "B", Version: 2, Origin: "node1", EventType: "WRITE",
	})
	
	res, _ = mgr.ComputeKeyResult("clusterT", "k1")
	// t=3: 2 distinct values (node1 has B, node2 has A)
	if res.ConvergenceCurve[2].Divergence != 2 {
		t.Fatalf("expected divergence 2 at t=3, got %d", res.ConvergenceCurve[2].Divergence)
	}
}
