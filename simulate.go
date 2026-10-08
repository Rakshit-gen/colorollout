package colorollout

import "time"

// Result is how one simulated rollout went.
type Result struct {
	State    State
	Took     time.Duration
	Extra    float64 // failed requests the release caused
	Peak     int     // most servers on the new version at once
	Revealed int     // stage where it was reverted or halted, -1 if done
	Events   []Event
}

// Simulate runs plan p on world w, one step of dt at a time, until the
// rollout ends or limit passes.
func Simulate(p Plan, g Gate, w *World, dt, limit time.Duration) Result {
	r := NewRollout(p, g, w.Fleet)
	w.Deploy(r.Next(w.Now()))
	res := Result{Revealed: -1}
	for r.State == Running && w.Now() < limit {
		smp := w.Step(dt)
		res.Peak = max(res.Peak, w.OnNew())
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
	res.State, res.Took, res.Extra, res.Events = r.State, w.Now(), w.Extra, r.Events
	return res
}
