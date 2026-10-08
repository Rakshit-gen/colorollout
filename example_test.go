package colorollout_test

import (
	"fmt"
	"time"

	"github.com/Rakshit-gen/colorollout"
)

func ExampleParseScope() {
	sc, _ := colorollout.ParseScope("tier=3 color=green traffic=free")
	f := colorollout.NewFleet([]int{2, 4, 8}, 6, 1)
	fmt.Println(sc, len(f.In(sc)), sc.Share())
	// Output: tier=3 color=green traffic=free 16 0.25
}

func ExampleGate_Judge() {
	slo := colorollout.SLO{Name: "5xx", Objective: 0.001}
	// Inside the SLO, but double the old version's ratio.
	v := colorollout.DefaultGate.Judge(slo,
		colorollout.Counts{Requests: 200000, Failures: 120},
		colorollout.Counts{Requests: 2000000, Failures: 600})
	fmt.Println(v.Decision, "-", v.Reason)
	// Output: revert - 5xx: 0.060% failing vs 0.030% on the old version (z=7.1)
}

func ExampleSimulate() {
	b := []byte(`{"service": "edge", "stages": [
		{"name": "small sites", "scope": "tier=3 color=green", "soak": "10m"},
		{"name": "everywhere", "scope": "everywhere", "soak": "10m"}],
		"slos": [{"name": "5xx", "objective": 0.001}]}`)
	p, err := colorollout.ParsePlan(b)
	if err != nil {
		panic(err)
	}
	f := colorollout.NewFleet([]int{4, 8, 12}, 6, 1)
	bug := colorollout.Raise{SLO: "5xx", Factor: 5}
	w := colorollout.NewWorld(f, map[string]float64{"5xx": 0.0003}, bug, 1)
	res := colorollout.Simulate(p, colorollout.DefaultGate, w, time.Minute, 24*time.Hour)
	fmt.Println(res.State, "at stage", res.Revealed+1)
	// Output: rolled back at stage 1
}
