package colorollout

import (
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
