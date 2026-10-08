package colorollout

import (
	"testing"
	"time"
)

func TestRaise(t *testing.T) {
	f := NewFleet([]int{1, 1}, 3, 1)
	b := Raise{SLO: "5xx", Factor: 10, Where: Scope{Tiers: []int{2}}}
	for _, s := range f.Servers {
		got := b.Ratio("5xx", s, 0.001, 0)
		want := 0.001
		if s.DC.Tier == 2 {
			want = 0.01
		}
		if got != want {
			t.Fatalf("server in tier %d: %v, want %v", s.DC.Tier, got, want)
		}
		if b.Ratio("timeouts", s, 0.001, 0) != 0.001 {
			t.Fatal("other SLO changed")
		}
	}
	if (Raise{SLO: "5xx", Factor: 5000}).Ratio("5xx", f.Servers[0], 0.001, 0) != 1 {
		t.Fatal("ratio not capped at 1")
	}
}

func TestCrashHitsEverySLO(t *testing.T) {
	f := NewFleet([]int{1}, 1, 1)
	b := Crash{Share: 0.5}
	for _, slo := range []string{"5xx", "timeouts"} {
		if got := b.Ratio(slo, f.Servers[0], 0, 0); got != 0.5 {
			t.Fatalf("%s: %v", slo, got)
		}
	}
	if got := b.Ratio("5xx", f.Servers[0], 0.1, 0); got != 0.55 {
		t.Fatalf("on top of a base ratio: %v", got)
	}
}

func TestSlowBurn(t *testing.T) {
	b := SlowBurn{SLO: "5xx", Delay: 30 * time.Minute, Ramp: 10 * time.Minute, Factor: 8}
	s := NewFleet([]int{1}, 1, 1).Servers[0]
	for _, c := range []struct {
		at   time.Duration
		want float64
	}{{0, 0.001}, {30 * time.Minute, 0.001}, {40 * time.Minute, 0.002}, {60 * time.Minute, 0.004}, {5 * time.Hour, 0.008}} {
		if got := b.Ratio("5xx", s, 0.001, c.at); got < c.want*0.999 || got > c.want*1.001 {
			t.Errorf("at %v: %v, want %v", c.at, got, c.want)
		}
	}
}

func TestUnderLoad(t *testing.T) {
	f := NewFleet([]int{2, 2, 2}, 6, 3)
	b := UnderLoad{SLO: "5xx", Above: 300, Factor: 20}
	hit := map[int]int{}
	for _, s := range f.Servers {
		if b.Ratio("5xx", s, 0.001, 0) > 0.001 {
			hit[s.DC.Tier]++
		}
	}
	if hit[1] == 0 || hit[2] != 0 || hit[3] != 0 {
		t.Fatalf("hits by tier: %v, want only tier 1", hit)
	}
}
