package colorollout

import (
	"testing"
	"time"
)

func TestWorldSplitsTrafficByVersion(t *testing.T) {
	f := NewFleet([]int{1, 2}, 6, 1)
	w := NewWorld(f, map[string]float64{"5xx": 0.001}, Raise{SLO: "5xx", Factor: 10}, 1)
	var total float64
	for _, s := range f.Servers {
		total += s.Load * 60
	}
	smp := w.Step(time.Minute)
	if smp.New["5xx"].Requests != 0 {
		t.Fatal("traffic on the new version before any deploy")
	}
	if got := float64(smp.Old["5xx"].Requests); got < total*0.97 || got > total*1.03 {
		t.Fatalf("requests %v, want about %v", got, total)
	}

	w.Deploy(f.In(Scope{Tiers: []int{2}}))
	var newC, oldC Counts
	for i := 0; i < 60; i++ {
		smp = w.Step(time.Minute)
		newC = newC.Add(smp.New["5xx"])
		oldC = oldC.Add(smp.Old["5xx"])
	}
	if r := newC.Ratio(); r < 0.008 || r > 0.012 {
		t.Fatalf("new version ratio %v, want about 0.01", r)
	}
	if r := oldC.Ratio(); r < 0.0008 || r > 0.0012 {
		t.Fatalf("old version ratio %v, want about 0.001", r)
	}
	if w.Extra <= 0 || w.Now() != 61*time.Minute {
		t.Fatalf("extra %v, now %v", w.Extra, w.Now())
	}
	w.Revert()
	if w.OnNew() != 0 || w.Step(time.Minute).New["5xx"].Requests != 0 {
		t.Fatal("revert left servers on the new version")
	}
}
