package colorollout

import (
	"testing"
	"time"
)

// A fleet about the size of the default backtest one.
func benchFleet() *Fleet { return NewFleet([]int{10, 20, 30}, 12, 1) }

func BenchmarkWorldStep(b *testing.B) {
	f := benchFleet()
	w := NewWorld(f, map[string]float64{"5xx": 0.0003}, nil, 1)
	w.Deploy(f.Servers[:len(f.Servers)/2], 1)
	for b.Loop() {
		w.Step(time.Minute)
	}
}

func BenchmarkObserve(b *testing.B) {
	p := loadPlan(b, "examples/edge-proxy.json")
	p.Stages[0].Soak = 1 << 62 // never leave the first stage
	r := NewRollout(p, DefaultGate, benchFleet())
	r.Next(0)
	smp := Sample{New: map[string]Counts{"5xx": {20000, 6}}, Old: map[string]Counts{"5xx": {2e6, 600}}}
	now := time.Duration(0)
	for b.Loop() {
		now += time.Minute
		r.Observe(now, smp)
	}
}

func BenchmarkSimulateGoodRelease(b *testing.B) {
	p := loadPlan(b, "examples/edge-proxy.json")
	f := benchFleet()
	for b.Loop() {
		Simulate(p, DefaultGate, NewWorld(f, map[string]float64{"5xx": 0.0003}, nil, 1), time.Minute, 48*time.Hour)
	}
}
