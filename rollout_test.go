package colorollout

import (
	"testing"
	"time"
)

func testPlan(t *testing.T) Plan {
	t.Helper()
	p := Plan{
		Service: "edge",
		SLOs:    []SLO{{"5xx", 0.001}},
		MaxWait: time.Hour,
	}
	for _, s := range []string{"tier=3 color=green", "tier=3", "everywhere"} {
		sc, err := ParseScope(s)
		if err != nil {
			t.Fatal(err)
		}
		p.Stages = append(p.Stages, Stage{Name: s, Scope: sc, Soak: 10 * time.Minute})
	}
	return p
}

func TestRolloutStateMachine(t *testing.T) {
	f := NewFleet([]int{2, 2, 4}, 6, 1)
	r := NewRollout(testPlan(t), DefaultGate, f)
	if s, _ := r.Next(0); len(s) != 8 {
		t.Fatalf("first stage %d servers, want 8", len(s))
	}
	healthy := Sample{
		New: map[string]Counts{"5xx": {20000, 10}},
		Old: map[string]Counts{"5xx": {200000, 100}},
	}
	if d := r.Observe(5*time.Minute, healthy); d != Wait {
		t.Fatalf("before the soak: %v", d)
	}
	if d := r.Observe(10*time.Minute, healthy); d != Continue {
		t.Fatalf("after the soak: %v", d)
	}
	r.Next(10 * time.Minute)
	if r.Canary["5xx"].Requests != 0 {
		t.Fatal("counts carried into the next stage")
	}
	bad := Sample{New: map[string]Counts{"5xx": {20000, 200}}, Old: healthy.Old}
	if d := r.Observe(11*time.Minute, bad); d != Revert || r.State != RolledBack {
		t.Fatalf("bad stage: %v, state %v", d, r.State)
	}
	if d := r.Observe(12*time.Minute, healthy); d != Wait {
		t.Fatal("observed after the rollout ended")
	}
}

func TestRolloutHaltsWithoutTraffic(t *testing.T) {
	r := NewRollout(testPlan(t), DefaultGate, NewFleet([]int{1, 1, 1}, 3, 1))
	r.Next(0)
	quiet := Sample{New: map[string]Counts{"5xx": {10, 0}}, Old: map[string]Counts{"5xx": {1000, 1}}}
	r.Observe(59*time.Minute, quiet)
	if r.State != Running {
		t.Fatal("halted before max wait")
	}
	r.Observe(time.Hour, quiet)
	if r.State != Halted {
		t.Fatalf("state %v after max wait without traffic", r.State)
	}
}

func TestRolloutFinishes(t *testing.T) {
	r := NewRollout(testPlan(t), DefaultGate, NewFleet([]int{1, 1, 1}, 3, 1))
	ok := Sample{New: map[string]Counts{"5xx": {20000, 10}}, Old: map[string]Counts{"5xx": {20000, 10}}}
	now := time.Duration(0)
	for i := range r.Plan.Stages {
		r.Next(now)
		now += 10 * time.Minute
		if d := r.Observe(now, ok); d != Continue {
			t.Fatalf("stage %d: %v", i, d)
		}
	}
	if r.State != Done {
		t.Fatalf("state %v", r.State)
	}
}

func TestWindowForgetsOldTraffic(t *testing.T) {
	p := testPlan(t)
	p.Window = 10 * time.Minute
	p.Stages[0].Soak = 2 * time.Hour
	r := NewRollout(p, DefaultGate, NewFleet([]int{1, 1, 1}, 3, 1))
	r.Next(0)
	ok := Sample{New: map[string]Counts{"5xx": {10000, 3}}, Old: map[string]Counts{"5xx": {10000, 3}}}
	for m := 1; m <= 90; m++ {
		r.Observe(time.Duration(m)*time.Minute, ok)
	}
	// Late trouble: 0.3% for a few minutes. Since the stage started that is
	// still well under 0.1% overall; in a 10 minute window it isn't.
	bad := Sample{New: map[string]Counts{"5xx": {10000, 30}}, Old: ok.Old}
	var d Decision
	for m := 91; m <= 94 && d != Revert; m++ {
		d = r.Observe(time.Duration(m)*time.Minute, bad)
	}
	if d != Revert {
		t.Fatalf("window missed late failures: %v", r.Canary)
	}
	if r.Canary["5xx"].Requests != 10*10000 {
		t.Fatalf("window holds %d requests, want 10 minutes' worth", r.Canary["5xx"].Requests)
	}
}
