package colorollout

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Stage is one step of a rollout: the new version goes to every server in
// Scope (on top of earlier stages) and has to stay healthy for Soak before
// the next stage starts.
type Stage struct {
	Name  string
	Scope Scope
	Soak  time.Duration
}

// Plan is a service's release plan: where the change goes, in what order,
// and how long it must look healthy at each step. Some services need long
// soaks; others accept false alarms to catch errors sooner. The plan is
// where a team says which.
type Plan struct {
	Service string
	Stages  []Stage
	SLOs    []SLO
	// MaxWait is how long a stage may go without enough traffic to judge
	// before the rollout stops and asks for a person.
	MaxWait time.Duration
	// Window is how far back the gate looks. Zero means since the stage
	// started.
	Window time.Duration
}

// Validate checks the plan can be run.
func (p Plan) Validate() error {
	var errs []error
	if p.Service == "" {
		errs = append(errs, errors.New("plan has no service name"))
	}
	if len(p.Stages) == 0 {
		errs = append(errs, errors.New("plan has no stages"))
	}
	if len(p.SLOs) == 0 {
		errs = append(errs, errors.New("plan has no SLOs: nothing would ever stop it"))
	}
	for i, s := range p.Stages {
		if s.Soak <= 0 {
			errs = append(errs, fmt.Errorf("stage %d (%s): soak must be positive", i+1, s.Name))
		}
	}
	if n := len(p.Stages); n > 0 && p.Stages[n-1].Scope.String() != "everywhere" {
		errs = append(errs, fmt.Errorf("last stage covers %q, not everywhere", p.Stages[n-1].Scope))
	}
	return errors.Join(errs...)
}

// planFile is the JSON form of a plan, with scopes and durations as strings.
type planFile struct {
	Service string `json:"service"`
	MaxWait string `json:"max_wait"`
	Window  string `json:"window"`
	Stages  []struct {
		Name  string `json:"name"`
		Scope string `json:"scope"`
		Soak  string `json:"soak"`
	} `json:"stages"`
	SLOs []SLO `json:"slos"`
}

// ParsePlan reads a plan from JSON:
//
//	{"service": "edge-proxy", "max_wait": "1h",
//	 "stages": [{"name": "canary", "scope": "dc=t3-eu-01", "soak": "10m"}, ...],
//	 "slos": [{"name": "5xx", "objective": 0.001}]}
//
// Unknown fields are an error, so a typo can't silently drop a safeguard.
func ParsePlan(b []byte) (Plan, error) {
	var f planFile
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return Plan{}, fmt.Errorf("plan: %w", err)
	}
	p := Plan{Service: f.Service, SLOs: f.SLOs}
	if f.MaxWait != "" {
		d, err := time.ParseDuration(f.MaxWait)
		if err != nil {
			return Plan{}, fmt.Errorf("plan: max_wait: %w", err)
		}
		p.MaxWait = d
	}
	if f.Window != "" {
		d, err := time.ParseDuration(f.Window)
		if err != nil || d < 0 {
			return Plan{}, fmt.Errorf("plan: window %q is not a positive duration", f.Window)
		}
		p.Window = d
	}
	for i, s := range f.Stages {
		sc, err := ParseScope(s.Scope)
		if err != nil {
			return Plan{}, fmt.Errorf("plan: stage %d: %w", i+1, err)
		}
		soak, err := time.ParseDuration(s.Soak)
		if err != nil {
			return Plan{}, fmt.Errorf("plan: stage %d: soak: %w", i+1, err)
		}
		p.Stages = append(p.Stages, Stage{Name: s.Name, Scope: sc, Soak: soak})
	}
	return p, p.Validate()
}
