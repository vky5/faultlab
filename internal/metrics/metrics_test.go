package metrics

import (
	"testing"
	"time"
)

func TestComputeDetectsVersionConflictNoConvergence(t *testing.T) {
	snapshots := []Snapshot{
		{
			Time: 0,
			Nodes: map[string]NodeState{
				"nodeA": {Value: "3", Version: 1, Origin: "nodeA", HasMetadata: true},
				"nodeB": {Value: "3", Version: 2, Origin: "nodeB", HasMetadata: true},
			},
		},
		{
			Time: 10 * time.Second,
			Nodes: map[string]NodeState{
				"nodeA": {Value: "3", Version: 1, Origin: "nodeA", HasMetadata: true},
				"nodeB": {Value: "3", Version: 2, Origin: "nodeB", HasMetadata: true},
			},
		},
	}

	result := Compute(snapshots)
	if result.ConvergenceTime != nil {
		t.Fatalf("expected no convergence, got %v", *result.ConvergenceTime)
	}
	if result.FinalConsistent {
		t.Fatalf("expected final inconsistency")
	}
	if result.PeakDivergence != 1 {
		t.Fatalf("unexpected peak divergence: %d", result.PeakDivergence)
	}
	if len(result.DivergenceOverTime) != 2 {
		t.Fatalf("unexpected divergence series length: %d", len(result.DivergenceOverTime))
	}
	if got := result.DivergenceOverTime[0].Divergence; got != 1 {
		t.Fatalf("unexpected first divergence: %d", got)
	}
	if got := result.DivergenceOverTime[1].Divergence; got != 1 {
		t.Fatalf("unexpected second divergence: %d", got)
	}
	if result.FirstAgreementTime != nil {
		t.Fatalf("expected no quorum agreement, got %v", *result.FirstAgreementTime)
	}

	if got := result.StaleDurationPerNode["nodeA"]; got != 0 {
		t.Fatalf("unexpected stale duration for nodeA: %v", got)
	}
	if got := result.StaleDurationPerNode["nodeB"]; got != 10*time.Second {
		t.Fatalf("unexpected stale duration for nodeB: %v", got)
	}
}

func TestComputeDetectsConvergenceAndStaleTime(t *testing.T) {
	snapshots := []Snapshot{
		{
			Time: 0,
			Nodes: map[string]NodeState{
				"nodeA": {Value: "3", Version: 1, Origin: "nodeA", HasMetadata: true},
				"nodeB": {Value: "3", Version: 2, Origin: "nodeB", HasMetadata: true},
				"nodeC": {Value: "3", Version: 2, Origin: "nodeB", HasMetadata: true},
			},
		},
		{
			Time: 5 * time.Second,
			Nodes: map[string]NodeState{
				"nodeA": {Value: "3", Version: 2, Origin: "nodeB", HasMetadata: true},
				"nodeB": {Value: "3", Version: 2, Origin: "nodeB", HasMetadata: true},
				"nodeC": {Value: "3", Version: 2, Origin: "nodeB", HasMetadata: true},
			},
		},
	}

	result := Compute(snapshots)
	if result.ConvergenceTime == nil || *result.ConvergenceTime != 5*time.Second {
		t.Fatalf("unexpected convergence time: %v", result.ConvergenceTime)
	}
	if !result.FinalConsistent {
		t.Fatalf("expected final consistency")
	}
	if result.PeakDivergence != 1 {
		t.Fatalf("unexpected peak divergence: %d", result.PeakDivergence)
	}
	if result.AreaUnderDivergence != 5*time.Second {
		t.Fatalf("unexpected area under curve: %v", result.AreaUnderDivergence)
	}
	if result.FirstAgreementTime == nil || *result.FirstAgreementTime != 0 {
		t.Fatalf("unexpected first agreement time: %v", result.FirstAgreementTime)
	}
	if got := result.StaleDurationPerNode["nodeA"]; got != 5*time.Second {
		t.Fatalf("unexpected stale duration for nodeA: %v", got)
	}
	if got := result.StaleDurationPerNode["nodeB"]; got != 0 {
		t.Fatalf("unexpected stale duration for nodeB: %v", got)
	}
	if got := result.StaleDurationPerNode["nodeC"]; got != 0 {
		t.Fatalf("unexpected stale duration for nodeC: %v", got)
	}
}
