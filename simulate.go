package colorollout

import "time"

// Result is how one simulated rollout went.
type Result struct {
	State    State
	Took     time.Duration
	Extra    float64 // failed requests the release caused
	Peak     float64 // largest share of requests on the new version in a step
	Revealed int     // stage where it was reverted or halted, -1 if done
	Events   []Event
}

// Baseline is how long Simulate watches the old version before releasing.
const Baseline = 30 * time.Minute

// Simulate runs plan p on world w, one step of dt at a time, until the
// rollout ends or limit passes. It first watches the fleet for Baseline to
// learn the old version's failure ratios. Took doesn't count that time.
func Simulate(p Plan, g Gate, w *World, dt, limit time.Duration) Result {
	return Drive(NewRollout(p, g, w.Fleet), w, dt, limit)
}

// Drive runs r on w for up to limit. A new rollout first watches for
// Baseline; one resumed from a journal already has its baseline, and gets
// the fleet put back the way the journal says it was.
func Drive(r *Rollout, w *World, dt, limit time.Duration) Result {
	if r.Baseline == nil {
		baseline := map[string]Counts{}
		for end := w.Now() + Baseline; w.Now() < end; {
			for k, c := range w.Step(dt).Old {
				baseline[k] = baseline[k].Add(c)
			}
		}
		r.SetBaseline(w.Now(), baseline)
	}
	start := w.Now()
	limit += start
	if r.Stage < 0 {
		w.Deploy(r.Next(w.Now()))
	} else {
		for s, share := range r.Covered() {
			w.Deploy([]*Server{s}, share)
		}
	}
	res := Result{Revealed: -1}
	for r.State == Running && w.Now() < limit {
		smp := w.Step(dt)
		res.Peak = max(res.Peak, newShare(smp))
		switch r.Observe(w.Now(), smp) {
		case Revert:
			w.Revert()
		case Continue:
			if r.State == Running {
				w.Deploy(r.Next(w.Now()))
			}
		}
	}
	if r.State != Done {
		res.Revealed = r.Stage
	}
	res.State, res.Took, res.Extra, res.Events = r.State, w.Now()-start, w.Extra, r.Events
	return res
}

func newShare(smp Sample) float64 {
	for k, n := range smp.New {
		if all := n.Requests + smp.Old[k].Requests; all > 0 {
			return float64(n.Requests) / float64(all)
		}
	}
	return 0
}
