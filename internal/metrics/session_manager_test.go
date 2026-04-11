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
		"nodeA": {Value: "3", Version: 1, Origin: "nodeA", HasMetadata: true},
		"nodeB": {Value: "3", Version: 2, Origin: "nodeB", HasMetadata: true},
		"nodeC": {Value: "3", Version: 1, Origin: "nodeA", HasMetadata: true},
	}); err != nil {
		t.Fatalf("record snapshot 1: %v", err)
	}

	if err := mgr.RecordSnapshot("clusterA", "k", start.Add(5*time.Second), map[string]NodeState{
		"nodeA": {Value: "3", Version: 2, Origin: "nodeB", HasMetadata: true},
		"nodeB": {Value: "3", Version: 2, Origin: "nodeB", HasMetadata: true},
		"nodeC": {Value: "3", Version: 2, Origin: "nodeB", HasMetadata: true},
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
