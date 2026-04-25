package gossip

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	logDir, err := os.MkdirTemp("", "faultlab-gossip-logs-*")
	if err == nil {
		_ = os.Setenv(gossipLogDirEnv, logDir)
	}
	code := m.Run()
	if logDir != "" {
		_ = os.RemoveAll(logDir)
	}
	os.Exit(code)
}

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

func TestAppendGossipFileLogWritesDateNamedFile(t *testing.T) {
	logDir := t.TempDir()
	t.Setenv(gossipLogDirEnv, logDir)

	if err := appendGossipFileLog("hello from gossip"); err != nil {
		t.Fatalf("appendGossipFileLog() error = %v", err)
	}

	logPath := filepath.Join(logDir, "gossip-"+time.Now().Format("2006-01-02")+".log")
	contents, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("expected gossip log file %s: %v", logPath, err)
	}
	if !strings.Contains(string(contents), "[gossip] hello from gossip") {
		t.Fatalf("expected log contents to contain message, got %q", string(contents))
	}
}

func TestPutUsesWallClockTimestamp(t *testing.T) {
	g := NewGossipProtocol()
	if err := g.Start("node1"); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	before := time.Now().UnixMilli()
	g.Put("k", "v")
	after := time.Now().UnixMilli()

	stored, ok := g.store["k"]
	if !ok {
		t.Fatalf("expected key to exist")
	}
	if stored.Timestamp < before || stored.Timestamp > after {
		t.Fatalf("expected wall-clock timestamp between %d and %d, got %d", before, after, stored.Timestamp)
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
