package colorollout

import (
	"math/rand/v2"
	"time"
)

// World simulates traffic on a fleet where some servers run the new version.
// The old version fails at Base for each SLO; the new one fails however Bug
// says.
type World struct {
	Fleet *Fleet
	Base  map[string]float64
	Bug   Bug

	rng   *rand.Rand
	now   time.Duration
	since map[int]time.Duration // server ID -> when it got the new version

	// Extra is failed requests caused by the release so far: failures on the
	// new version beyond what the old version would have had. It's the
	// blast radius a rollout is trying to keep small.
	Extra float64
}

// NewWorld starts with every server on the old version.
func NewWorld(f *Fleet, base map[string]float64, bug Bug, seed uint64) *World {
	if bug == nil {
		bug = NoBug{}
	}
	return &World{Fleet: f, Base: base, Bug: bug, rng: rand.New(rand.NewPCG(seed, 11)), since: map[int]time.Duration{}}
}

// Now is simulated time since the start.
func (w *World) Now() time.Duration { return w.now }

// Deploy puts the new version on servers that don't have it yet.
func (w *World) Deploy(servers []*Server) {
	for _, s := range servers {
		if _, ok := w.since[s.ID]; !ok {
			w.since[s.ID] = w.now
		}
	}
}

// Revert puts every server back on the old version.
func (w *World) Revert() { clear(w.since) }

// OnNew reports how many servers run the new version.
func (w *World) OnNew() int { return len(w.since) }

// Sample is one step's counts per SLO, split by version.
type Sample struct {
	New, Old map[string]Counts
}

// Step advances time by dt and returns the traffic seen in it.
func (w *World) Step(dt time.Duration) Sample {
	out := Sample{New: map[string]Counts{}, Old: map[string]Counts{}}
	for _, s := range w.Fleet.Servers {
		n := poisson(w.rng, s.Load*dt.Seconds())
		start, isNew := w.since[s.ID]
		for slo, base := range w.Base {
			ratio := base
			if isNew {
				ratio = w.Bug.Ratio(slo, s, base, w.now-start)
				w.Extra += float64(n) * (ratio - base)
			}
			c := Counts{n, poisson(w.rng, float64(n)*ratio)}
			if isNew {
				out.New[slo] = out.New[slo].Add(c)
			} else {
				out.Old[slo] = out.Old[slo].Add(c)
			}
		}
	}
	w.now += dt
	return out
}
