package colorollout

import (
	"strings"
	"testing"
	"time"
)

func TestValidate(t *testing.T) {
	ok := Plan{
		Service: "edge-proxy",
		Stages: []Stage{
			{Name: "canary", Scope: Scope{Tiers: []int{3}, Colors: []string{"green"}}, Soak: 10 * time.Minute},
			{Name: "all", Soak: 30 * time.Minute},
		},
		SLOs: []SLO{{Name: "5xx", Objective: 0.001}},
	}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	bad := ok
	bad.SLOs = nil
	bad.Stages = []Stage{{Name: "canary", Scope: Scope{Tiers: []int{3}}}}
	err := bad.Validate()
	if err == nil {
		t.Fatal("bad plan accepted")
	}
	for _, want := range []string{"no SLOs", "soak", "not everywhere"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}
