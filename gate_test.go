package colorollout

import "testing"

func TestGate(t *testing.T) {
	slo := SLO{"5xx", 0.001}
	g := DefaultGate
	cases := []struct {
		name            string
		canary, control Counts
		want            Decision
	}{
		{"too little traffic", Counts{500, 0}, Counts{1e6, 100}, Wait},
		{"healthy", Counts{100000, 50}, Counts{1e6, 500}, Continue},
		{"over the SLO", Counts{100000, 300}, Counts{1e6, 500}, Revert},
		{"inside the SLO but worse than before", Counts{200000, 120}, Counts{1e6, 100}, Revert},
		{"over the SLO by noise only", Counts{2000, 3}, Counts{1e6, 1000}, Continue},
	}
	for _, c := range cases {
		if v := g.Judge(slo, c.canary, c.control); v.Decision != c.want {
			t.Errorf("%s: got %v (%s), want %v", c.name, v.Decision, v.Reason, c.want)
		}
	}
}
