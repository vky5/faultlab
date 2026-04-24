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
