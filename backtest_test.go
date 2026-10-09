package colorollout

import (
	"os"
	"testing"
)

func loadPlan(t testing.TB, path string) Plan {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	p, err := ParsePlan(b)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestBacktestStagedBeatsBigBang(t *testing.T) {
	f := NewFleet([]int{10, 20, 30}, 12, 1)
	plans := []Plan{loadPlan(t, "examples/big-bang.json"), loadPlan(t, "examples/edge-proxy.json")}
	inc := []Incident{Incidents[0], Incidents[len(Incidents)-1]}
	out := Backtest(plans, inc, f, map[string]float64{"5xx": 0.0003}, 3)
	if len(out) != 4 {
		t.Fatalf("%d outcomes", len(out))
	}
	bang, staged := out[0], out[2]
	if bang.Caught != 3 || staged.Caught != 3 {
		t.Fatalf("x10 errors not always caught: %+v %+v", bang, staged)
	}
	if staged.Extra*10 > bang.Extra {
		t.Fatalf("staged caused %.0f failures, big bang %.0f", staged.Extra, bang.Extra)
	}
	for _, good := range []Outcome{out[1], out[3]} {
		if good.Caught != 0 {
			t.Fatalf("%s reverted a good release", good.Plan)
		}
	}
}

func TestBacktestNoSeeds(t *testing.T) {
	f := NewFleet([]int{10}, 2, 1)
	out := Backtest([]Plan{loadPlan(t, "examples/big-bang.json")}, Incidents[:1], f, nil, 0)
	if len(out) != 1 || out[0].Runs != 0 || out[0].Extra != 0 || out[0].Took != 0 {
		t.Fatalf("%+v", out)
	}
}
