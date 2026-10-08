package colorollout

import (
	"os"
	"path/filepath"
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

func TestParsePlanExample(t *testing.T) {
	b, err := os.ReadFile("examples/edge-proxy.json")
	if err != nil {
		t.Fatal(err)
	}
	p, err := ParsePlan(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Stages) != 5 || p.Stages[1].Soak != 20*time.Minute || p.MaxWait != time.Hour || p.SLOs[0].Objective != 0.001 {
		t.Fatalf("%+v", p)
	}
	f := NewFleet([]int{10, 20, 30}, 12, 1)
	if n := len(f.In(p.Stages[0].Scope)); n != 4 {
		t.Fatalf("first stage has %d servers", n)
	}
}

func TestParsePlanRejectsTypos(t *testing.T) {
	_, err := ParsePlan([]byte(`{"service": "x", "stages": [], "slo": []}`))
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("got %v", err)
	}
}

func TestParsePlanWindow(t *testing.T) {
	b, err := os.ReadFile("examples/edge-proxy.json")
	if err != nil {
		t.Fatal(err)
	}
	p, err := ParsePlan(b)
	if err != nil {
		t.Fatal(err)
	}
	if p.Window != 10*time.Minute {
		t.Fatalf("window %v", p.Window)
	}
	if _, err := ParsePlan([]byte(`{"service":"x","window":"-5m"}`)); err == nil {
		t.Fatal("negative window accepted")
	}
}

func TestExamplesParse(t *testing.T) {
	paths, _ := filepath.Glob("examples/*.json")
	if len(paths) < 5 {
		t.Fatalf("found %d examples", len(paths))
	}
	f := NewFleet([]int{10, 20, 30}, 12, 1)
	for _, path := range paths {
		p := loadPlan(t, path)
		for i, st := range p.Stages {
			if len(f.In(st.Scope)) == 0 {
				t.Errorf("%s stage %d matches nothing in the default fleet", path, i+1)
			}
		}
	}
}
