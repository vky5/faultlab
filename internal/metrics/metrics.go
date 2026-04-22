package metrics

import (
	"sort"
	"time"
)

// NodeState is the observable state for one tracked key on one node.
// Equality is strict: existence, value, version, origin, and metadata flag all matter.
type NodeState struct {
	Exists      bool
	Value       string
	Version     int64
	Origin      string
	HasMetadata bool
}

// Snapshot captures the observed state of all nodes at one logical time.
type Snapshot struct {
	Time  time.Duration
	Nodes map[string]NodeState
}

// TimelineEvent is a fine-grained state change event.
type TimelineEvent struct {
	Time      time.Duration
	NodeID    string
	Key       string
	Value     string
	Version   int64
	Origin    string
	Source    string
	EventType    string // WRITE, GOSSIP_RECEIVE, RESOLVE
	Round        uint64 // logical tick or gossip cycle
	LWWTimestamp int64  // high-resolution wall clock for LWW
}

/*
Snapshot{
    Time: 5 * time.Second,
    Nodes: map[string]NodeState{
        "nodeA": {Value: "3", Version: 1, Origin: "nodeA", HasMetadata: true},
        "nodeB": {Value: "3", Version: 2, Origin: "nodeB", HasMetadata: true},
    },
}
*/

// DivergencePoint records the divergence at a particular snapshot time.
type DivergencePoint struct {
	Time       time.Duration
	Divergence int
}

// Result is the aggregated metrics output for a sequence of snapshots.
type Result struct {
	ConvergenceTime      *time.Duration           // first time when all nodes have identical state
	FirstAgreementTime   *time.Duration           // first time when a quorum of nodes agree on the same state
	FinalConsistent      bool                     // whether all nodes are consistent in the final snapshot
	PeakDivergence       int                      // maximum number of divergent nodes at any snapshot
	AreaUnderDivergence  time.Duration            //
	DivergenceOverTime   []DivergencePoint        // time series of divergence at each snapshot
	StaleDurationPerNode map[string]time.Duration // total time each node was in a divergent state
	Timeline             []TimelineEvent          // fine-grained event history
	ConvergenceCurve     []DivergencePoint        // number of distinct values over time
}

// Compute reduces ordered or unordered snapshots into the core metrics.
func Compute(snapshots []Snapshot) Result {
	result := Result{
		StaleDurationPerNode: make(map[string]time.Duration),
	}
	if len(snapshots) == 0 {
		return result
	}

	ordered := make([]Snapshot, len(snapshots))
	copy(ordered, snapshots)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].Time < ordered[j].Time
	})

	for i, snapshot := range ordered {
		mode, modeCount := mostCommonState(snapshot.Nodes)
		divergence := len(snapshot.Nodes) - modeCount

		result.DivergenceOverTime = append(result.DivergenceOverTime, DivergencePoint{
			Time:       snapshot.Time,
			Divergence: divergence,
		})

		if divergence > result.PeakDivergence {
			result.PeakDivergence = divergence
		}

		if divergence == 0 && result.ConvergenceTime == nil {
			if allNodesExist(snapshot.Nodes) {
				convergenceTime := snapshot.Time
				result.ConvergenceTime = &convergenceTime
			}
		}

		if result.FirstAgreementTime == nil {
			quorum := (len(snapshot.Nodes) / 2) + 1
			if modeCount >= quorum && quorum > 0 {
				agreementTime := snapshot.Time
				result.FirstAgreementTime = &agreementTime
			}
		}

		if i < len(ordered)-1 {
			delta := ordered[i+1].Time - snapshot.Time
			if delta < 0 {
				delta = 0
			}
			result.AreaUnderDivergence += time.Duration(divergence) * delta
			for nodeID, nodeState := range snapshot.Nodes {
				if nodeState != mode {
					result.StaleDurationPerNode[nodeID] += delta
				}
			}
		}
	}

	last := result.DivergenceOverTime[len(result.DivergenceOverTime)-1]
	result.FinalConsistent = last.Divergence == 0 && allNodesExist(ordered[len(ordered)-1].Nodes)
	return result
}

func allNodesExist(nodes map[string]NodeState) bool {
	if len(nodes) == 0 {
		return false
	}
	for _, node := range nodes {
		if !node.Exists {
			return false
		}
	}
	return true
}

func mostCommonState(nodes map[string]NodeState) (NodeState, int) {
	if len(nodes) == 0 {
		return NodeState{}, 0
	}

	counts := make(map[NodeState]int, len(nodes))
	var topState NodeState
	topCount := 0

	for _, state := range nodes {
		counts[state]++
		count := counts[state]
		if count > topCount || (count == topCount && lessState(state, topState)) {
			topState = state
			topCount = count
		}
	}

	return topState, topCount
}

func lessState(a, b NodeState) bool {
	if a.Value != b.Value {
		return a.Value < b.Value
	}
	if a.Version != b.Version {
		return a.Version < b.Version
	}
	if a.Origin != b.Origin {
		return a.Origin < b.Origin
	}
	if a.HasMetadata != b.HasMetadata {
		return !a.HasMetadata && b.HasMetadata
	}
	return false
}
