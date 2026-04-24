package gossip

import "testing"

func TestVectorClockCompare(t *testing.T) {
	tests := []struct {
		name string
		vc1  map[string]int64
		vc2  map[string]int64
		want int
	}{
		{
			name: "equal nil clocks",
			want: 0,
		},
		{
			name: "equal empty clocks",
			vc1:  map[string]int64{},
			vc2:  map[string]int64{},
			want: 0,
		},
		{
			name: "missing entries compare as zero",
			vc1:  map[string]int64{},
			vc2:  map[string]int64{"node-a": 0},
			want: 0,
		},
		{
			name: "vc1 causally after vc2",
			vc1:  map[string]int64{"node-a": 2, "node-b": 1},
			vc2:  map[string]int64{"node-a": 1, "node-b": 1},
			want: 1,
		},
		{
			name: "vc1 causally before vc2",
			vc1:  map[string]int64{"node-a": 1},
			vc2:  map[string]int64{"node-a": 1, "node-b": 1},
			want: -1,
		},
		{
			name: "concurrent clocks",
			vc1:  map[string]int64{"node-a": 2, "node-b": 1},
			vc2:  map[string]int64{"node-a": 1, "node-b": 2},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := vectorClockCompare(tt.vc1, tt.vc2); got != tt.want {
				t.Fatalf("vectorClockCompare(%v, %v) = %d, want %d", tt.vc1, tt.vc2, got, tt.want)
			}
		})
	}
}

func TestPutUsesLogicalTickTimestamp(t *testing.T) {
	g := NewGossipProtocol()
	if err := g.Start("node1"); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Advance logical time so the write timestamp reflects protocol time, not wall clock.
	g.tick = 7
	g.Put("k", "v")

	stored, ok := g.store["k"]
	if !ok {
		t.Fatalf("expected key to exist")
	}
	if stored.Timestamp != 7 {
		t.Fatalf("expected logical timestamp 7, got %d", stored.Timestamp)
	}
}

func TestConcurrentWritesTieBreakByOriginWhenLogicalTimeEqual(t *testing.T) {
	local := Value{
		Data:        "groupA_update",
		Version:     2,
		Origin:      "node1",
		Timestamp:   7,
		VectorClock: map[string]int64{"node1": 2},
	}
	incoming := Value{
		Data:        "groupB_update",
		Version:     2,
		Origin:      "node4",
		Timestamp:   7,
		VectorClock: map[string]int64{"node4": 2},
	}

	if !isIncomingNewer(local, incoming) {
		t.Fatalf("expected incoming concurrent value with higher origin to win")
	}
}
