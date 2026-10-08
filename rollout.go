package colorollout

import (
	"fmt"
	"time"
)

// State is where a rollout is.
type State int

const (
	Running    State = iota
	Done             // every stage passed
	RolledBack       // health checks failed and the change was reverted
	Halted           // couldn't judge in time; waiting for a person
)

func (s State) String() string {
	return [...]string{"running", "done", "rolled back", "halted"}[s]
}

// Event is one line of a rollout's history.
type Event struct {
	At    time.Duration
	Stage int
	What  string
}

func (e Event) String() string {
	return fmt.Sprintf("%8v  stage %d  %s", e.At.Round(time.Second), e.Stage+1, e.What)
}

// Rollout runs a plan. It doesn't touch servers itself: Next tells the
// caller which servers to put on the new version, and Observe takes in
// traffic and says whether to keep going, revert, or stop.
type Rollout struct {
	Plan  Plan
	Gate  Gate
	Fleet *Fleet

	State      State
	Stage      int
	StageStart time.Duration
	Canary     map[string]Counts // new version since the stage started
	Control    map[string]Counts // old version over the same time
	Events     []Event
}

// NewRollout prepares a plan for the fleet. Call Next to start.
func NewRollout(p Plan, g Gate, f *Fleet) *Rollout {
	return &Rollout{Plan: p, Gate: g, Fleet: f, Stage: -1}
}

func (r *Rollout) log(at time.Duration, format string, args ...any) {
	r.Events = append(r.Events, Event{at, r.Stage, fmt.Sprintf(format, args...)})
}

// Next moves to the next stage and returns the servers it adds.
func (r *Rollout) Next(now time.Duration) []*Server {
	r.Stage++
	r.StageStart = now
	r.Canary, r.Control = map[string]Counts{}, map[string]Counts{}
	st := r.Plan.Stages[r.Stage]
	servers := r.Fleet.In(st.Scope)
	r.log(now, "deploy to %s (%d servers), soak %v", st.Scope, len(servers), st.Soak)
	return servers
}

// Observe adds traffic seen up to now and returns what the gate decided.
// On Revert the rollout is over and the caller must put every server back
// on the old version. On Continue past the last stage it is done.
func (r *Rollout) Observe(now time.Duration, smp Sample) Decision {
	if r.State != Running {
		return Wait
	}
	for k, c := range smp.New {
		r.Canary[k] = r.Canary[k].Add(c)
	}
	for k, c := range smp.Old {
		r.Control[k] = r.Control[k].Add(c)
	}
	v := r.Gate.JudgeAll(r.Plan.SLOs, r.Canary, r.Control)
	elapsed := now - r.StageStart
	switch {
	case v.Decision == Revert:
		r.State = RolledBack
		r.log(now, "revert: %s", v.Reason)
		return Revert
	case v.Decision == Continue && elapsed >= r.Plan.Stages[r.Stage].Soak:
		r.log(now, "healthy after %v", elapsed)
		if r.Stage == len(r.Plan.Stages)-1 {
			r.State = Done
			r.log(now, "done")
		}
		return Continue
	case v.Decision == Wait && r.Plan.MaxWait > 0 && elapsed >= r.Plan.MaxWait:
		r.State = Halted
		r.log(now, "halt: %s after %v; needs a person", v.Reason, elapsed)
	}
	return Wait
}
