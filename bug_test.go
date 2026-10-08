package colorollout

import "testing"

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
