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

func TestJudgeAllWorstWins(t *testing.T) {
	slos := []SLO{{"5xx", 0.001}, {"timeouts", 0.01}}
	control := map[string]Counts{"5xx": {1e6, 500}, "timeouts": {1e6, 5000}}
	good := map[string]Counts{"5xx": {1e5, 50}, "timeouts": {1e5, 500}}
	if v := DefaultGate.JudgeAll(slos, good, control); v.Decision != Continue {
		t.Fatalf("all good: %v", v)
	}
	slowOnly := map[string]Counts{"5xx": {1e5, 50}, "timeouts": {1e5, 2000}}
	if v := DefaultGate.JudgeAll(slos, slowOnly, control); v.Decision != Revert || v.Reason[:8] != "timeouts" {
		t.Fatalf("one SLO bad: %v", v)
	}
	missing := map[string]Counts{"5xx": {1e5, 50}}
	if v := DefaultGate.JudgeAll(slos, missing, control); v.Decision != Wait {
		t.Fatalf("one SLO without data: %v", v)
	}
}
