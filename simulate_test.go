package colorollout

import (
	"testing"
	"time"
)

var base = map[string]float64{"5xx": 0.0003}

func TestSimulateGoodRelease(t *testing.T) {
	f := NewFleet([]int{4, 8, 12}, 6, 1)
	res := Simulate(testPlan(t), DefaultGate, NewWorld(f, base, nil, 1), time.Minute, 24*time.Hour)
	if res.State != Done || res.Peak != 1 {
		t.Fatalf("good release: %v, peak %v\n%v", res.State, res.Peak, res.Events)
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
	if res.Peak >= 0.25 {
		t.Fatalf("bad version reached %.0f%% of traffic", 100*res.Peak)
	}
}

func TestDriveResumed(t *testing.T) {
	f := NewFleet([]int{4, 8, 12}, 6, 1)
	p := testPlan(t)
	first := Simulate(p, DefaultGate, NewWorld(f, base, nil, 1), time.Minute, 15*time.Minute)
	if first.State != Running {
		t.Fatalf("stopped early run: %v", first.State)
	}
	w := NewWorld(f, base, nil, 2)
	last := first.Events[len(first.Events)-1].At
	w.Skip(last)
	r := Resume(p, DefaultGate, f, first.Events, w.Now())
	res := Drive(r, w, time.Minute, 24*time.Hour)
	if res.State != Done {
		t.Fatalf("resumed run: %v\n%v", res.State, res.Events)
	}
	if res.Took != 20*time.Minute { // stage 2 soaks again from the start, then stage 3
		t.Fatalf("resumed run took %v", res.Took)
	}
}
