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
	At    time.Duration `json:"at"`
	Stage int           `json:"stage"`
	Kind  string        `json:"kind"` // deploy, healthy, revert, halt, done or resume
	What  string        `json:"what"`
}

func (e Event) String() string {
	return fmt.Sprintf("%8v  stage %d  %-7s  %s", e.At.Round(time.Second), e.Stage+1, e.Kind, e.What)
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
	Canary     map[string]Counts // new version over the window
	Control    map[string]Counts // old version over the same time
	Events     []Event

	// Page, if set, is called when the rollout reverts or halts. The revert
	// has already happened by then; the page is so someone looks at why.
	Page func(Event)
	// Journal, if set, gets every event before the rollout acts on it. If
	// it can't be written the rollout halts rather than go on unrecorded.
	Journal *Journal

	// Freezes are times when no new stage may start, such as a big event
	// or a holiday. A frozen rollout keeps watching and can still revert.
	Freezes []Freeze
	held    bool
	strikes int // revert verdicts in a row

	recent []stamped
}

// Freeze is a span of time when rollouts hold where they are.
type Freeze struct{ From, To time.Duration }

func (r *Rollout) frozen(now time.Duration) bool {
	for _, f := range r.Freezes {
		if now >= f.From && now < f.To {
			return true
		}
	}
	return false
}

type stamped struct {
	at time.Duration
	Sample
}

// NewRollout prepares a plan for the fleet. Call Next to start.
func NewRollout(p Plan, g Gate, f *Fleet) *Rollout {
	return &Rollout{Plan: p, Gate: g, Fleet: f, Stage: -1}
}

func (r *Rollout) log(at time.Duration, kind, format string, args ...any) {
	e := Event{at, r.Stage, kind, fmt.Sprintf(format, args...)}
	r.Events = append(r.Events, e)
	if r.Journal == nil {
		return
	}
	if err := r.Journal.Append(e); err != nil && r.State == Running {
		r.State = Halted
		r.Events = append(r.Events, Event{at, r.Stage, "halt", "journal: " + err.Error()})
		r.page()
	}
}

func (r *Rollout) page() {
	if r.Page != nil {
		r.Page(r.Events[len(r.Events)-1])
	}
}

// Next moves to the next stage and returns the servers it covers and the
// share of their traffic that should get the new version.
func (r *Rollout) Next(now time.Duration) ([]*Server, float64) {
	r.Stage++
	r.StageStart = now
	r.Canary, r.Control = map[string]Counts{}, map[string]Counts{}
	r.recent = r.recent[:0]
	st := r.Plan.Stages[r.Stage]
	servers := r.Fleet.In(st.Scope)
	r.log(now, "deploy", "deploy to %s (%d servers), soak %v", st.Scope, len(servers), st.Soak)
	if r.State != Running {
		return nil, 0
	}
	return servers, st.Scope.Share()
}

// Observe adds traffic seen up to now and returns what the gate decided.
// On Revert the rollout is over and the caller must put every server back
// on the old version. On Continue past the last stage it is done.
func (r *Rollout) Observe(now time.Duration, smp Sample) Decision {
	if r.State != Running {
		return Wait
	}
	r.window(now, smp)
	v := r.Gate.JudgeAll(r.Plan.SLOs, r.Canary, r.Control)
	elapsed := now - r.StageStart
	if v.Decision == Revert {
		r.strikes++
		if r.strikes < r.Gate.Confirm {
			return Wait
		}
	} else {
		r.strikes = 0
	}
	switch {
	case v.Decision == Revert:
		r.State = RolledBack
		r.log(now, "revert", "%s", v.Reason)
		r.page()
		return Revert
	case v.Decision == Continue && elapsed >= r.Plan.Stages[r.Stage].Soak:
		if r.Stage < len(r.Plan.Stages)-1 && r.frozen(now) {
			if !r.held {
				r.log(now, "hold", "healthy, but in a change freeze")
				r.held = true
			}
			return Wait
		}
		r.held = false
		r.log(now, "healthy", "healthy after %v", elapsed)
		if r.Stage == len(r.Plan.Stages)-1 {
			r.State = Done
			r.log(now, "done", "every stage passed")
		}
		return Continue
	case v.Decision == Wait && r.Plan.MaxWait > 0 && elapsed >= r.Plan.MaxWait && !r.held:
		r.State = Halted
		r.log(now, "halt", "%s after %v; needs a person", v.Reason, elapsed)
		r.page()
	}
	return Wait
}

// window adds smp and recounts Canary and Control over the plan's window,
// or since the stage started when the plan has none. A window like HMD's
// rate(...[10m]) shows a problem that starts late in a long soak at full
// strength instead of diluted by the healthy hours before it.
func (r *Rollout) window(now time.Duration, smp Sample) {
	r.recent = append(r.recent, stamped{now, smp})
	if w := r.Plan.Window; w > 0 {
		i := 0
		for i < len(r.recent) && r.recent[i].at <= now-w {
			i++
		}
		r.recent = r.recent[i:]
	}
	clear(r.Canary)
	clear(r.Control)
	for _, x := range r.recent {
		for k, c := range x.New {
			r.Canary[k] = r.Canary[k].Add(c)
		}
		for k, c := range x.Old {
			r.Control[k] = r.Control[k].Add(c)
		}
	}
}

// Covered is what the fleet should look like right now: each server that
// should run the new version, with its share of traffic. After a crash this
// is what a resumed rollout checks the fleet against. A reverted rollout
// covers nothing; a halted one keeps what it had until a person decides.
func (r *Rollout) Covered() map[*Server]float64 {
	out := map[*Server]float64{}
	if r.State == RolledBack {
		return out
	}
	for i := 0; i <= r.Stage && i < len(r.Plan.Stages); i++ {
		sc := r.Plan.Stages[i].Scope
		for _, s := range r.Fleet.In(sc) {
			out[s] = max(out[s], sc.Share())
		}
	}
	return out
}
