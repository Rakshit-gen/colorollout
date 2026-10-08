package colorollout

import (
	"testing"
	"time"
)

var base = map[string]float64{"5xx": 0.0003}

func TestSimulateGoodRelease(t *testing.T) {
	f := NewFleet([]int{4, 8, 12}, 6, 1)
	res := Simulate(testPlan(t), DefaultGate, NewWorld(f, base, nil, 1), time.Minute, 24*time.Hour)
	if res.State != Done || res.Peak != len(f.Servers) {
		t.Fatalf("good release: %v, peak %d\n%v", res.State, res.Peak, res.Events)
	}
	if res.Took != 30*time.Minute {
		t.Fatalf("took %v, want three 10m soaks", res.Took)
	}
}

func TestSimulateBadReleaseStopsEarly(t *testing.T) {
	f := NewFleet([]int{4, 8, 12}, 6, 1)
	w := NewWorld(f, base, Raise{SLO: "5xx", Factor: 20}, 1)
	res := Simulate(testPlan(t), DefaultGate, w, time.Minute, 24*time.Hour)
	if res.State != RolledBack || res.Revealed != 0 {
		t.Fatalf("bad release: %v at stage %d\n%v", res.State, res.Revealed, res.Events)
	}
	if w.OnNew() != 0 {
		t.Fatal("servers left on the bad version")
	}
	if res.Peak >= len(f.Servers)/4 {
		t.Fatalf("bad version reached %d of %d servers", res.Peak, len(f.Servers))
	}
}
