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
	since map[int]deployment // server ID -> its share of the new version

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
	return &World{Fleet: f, Base: base, Bug: bug, rng: rand.New(rand.NewPCG(seed, 11)), since: map[int]deployment{}}
}

type deployment struct {
	at    time.Duration // when the server first got the new version
	share float64       // fraction of its requests on the new version
}

// Now is simulated time since the start.
func (w *World) Now() time.Duration { return w.now }

// Skip moves the clock to t without simulating traffic, for picking up a
// journal where its times left off.
func (w *World) Skip(t time.Duration) { w.now = max(w.now, t) }

// Deploy sends share of each server's requests to the new version. A
// server already on a bigger share keeps it.
func (w *World) Deploy(servers []*Server, share float64) {
	for _, s := range servers {
		d, ok := w.since[s.ID]
		if !ok {
			d.at = w.now
		}
		d.share = max(d.share, share)
		w.since[s.ID] = d
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
		d := w.since[s.ID]
		mean := s.Load * dt.Seconds()
		nNew := poisson(w.rng, mean*d.share)
		nOld := poisson(w.rng, mean*(1-d.share))
		for slo, base := range w.Base {
			if nOld > 0 {
				out.Old[slo] = out.Old[slo].Add(Counts{nOld, poisson(w.rng, float64(nOld)*base)})
			}
			if nNew > 0 {
				ratio := w.Bug.Ratio(slo, s, base, w.now-d.at)
				w.Extra += float64(nNew) * (ratio - base)
				out.New[slo] = out.New[slo].Add(Counts{nNew, poisson(w.rng, float64(nNew)*ratio)})
			}
		}
	}
	w.now += dt
	return out
}
