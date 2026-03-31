package fault

import (
"testing"
"time"
)

func TestBeforeProbeReturnsDelayDecision(t *testing.T) {
	fe := NewEngine()
	fe.SetDelay(40)

	d := fe.BeforeProbe("peer-1")
	if !d.Allow {
		t.Fatalf("expected probe to be allowed")
	}
	if d.Delay != 40*time.Millisecond {
		t.Fatalf("expected probe delay=40ms, got %v", d.Delay)
	}
}

func TestBeforeProbeStillBlocksWhenPartitioned(t *testing.T) {
	fe := NewEngine()
	fe.SetDelay(50)
	fe.Partition("peer-2")

	d := fe.BeforeProbe("peer-2")
	if d.Allow {
		t.Fatalf("expected probe to be blocked for partitioned peer")
	}
	if d.Delay != 0 {
		t.Fatalf("expected blocked probe delay=0, got %v", d.Delay)
	}
}

func TestBeforeTickReturnsDelayWhenAllowed(t *testing.T) {
	fe := NewEngine()
	fe.SetDelay(25)

	d := fe.BeforeTick()
	if !d.Allow {
		t.Fatalf("expected tick to be allowed")
	}
	if d.Delay != 25*time.Millisecond {
		t.Fatalf("expected tick delay=25ms, got %v", d.Delay)
	}
}
