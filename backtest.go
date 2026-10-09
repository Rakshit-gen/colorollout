package colorollout

import (
	"time"
)

// Incident is a kind of bad release to replay against a plan.
type Incident struct {
	Key  string // short name for the command line
	Name string
	Bug  Bug
}

// Incidents are the profiles the backtest replays. They are made up, shaped
// after the kinds of failures staged rollouts exist for; the last one is a
// good release, to count false alarms.
var Incidents = []Incident{
	{"x10", "errors x10 everywhere", Raise{SLO: "5xx", Factor: 10}},
	{"x3", "errors x3 everywhere", Raise{SLO: "5xx", Factor: 3}},
	{"x1.5", "errors x1.5 everywhere", Raise{SLO: "5xx", Factor: 1.5}},
	{"tier1", "errors x10 on tier 1 only", Raise{SLO: "5xx", Factor: 10, Where: Scope{Tiers: []int{1}}}},
	{"crash", "crash 20%", Crash{Share: 0.2}},
	{"slowburn", "slow burn after 40m", SlowBurn{SLO: "5xx", Delay: 40 * time.Minute, Ramp: 10 * time.Minute, Factor: 20}},
	{"load", "errors x10 above 300 rps", UnderLoad{SLO: "5xx", Above: 300, Factor: 10}},
	{"good", "good release", NoBug{}},
}

// Outcome is one plan against one incident, over several seeds.
type Outcome struct {
	Plan, Incident string
	Runs           int
	Caught         int           // runs that reverted or halted, not ones cut off by the time limit
	Detect         time.Duration // mean time to revert, over caught runs
	Extra          float64       // mean failed requests caused
	Peak           float64       // mean largest share of traffic that got the change
	Took           time.Duration // mean time until the rollout ended
}

// Backtest runs every plan against every incident, seeds times each, on a
// fresh copy of the same fleet. base is the old version's failure ratio per
// SLO.
func Backtest(plans []Plan, incidents []Incident, f *Fleet, base map[string]float64, seeds int) []Outcome {
	var out []Outcome
	for _, p := range plans {
		for _, inc := range incidents {
			o := Outcome{Plan: p.Service, Incident: inc.Name, Runs: seeds}
			for seed := range seeds {
				w := NewWorld(f, base, inc.Bug, uint64(seed+1))
				res := Simulate(p, DefaultGate, w, time.Minute, 48*time.Hour)
				if res.State == RolledBack || res.State == Halted {
					o.Caught++
					o.Detect += res.Took
				}
				o.Extra += res.Extra
				o.Peak += res.Peak
				o.Took += res.Took
			}
			if o.Caught > 0 {
				o.Detect /= time.Duration(o.Caught)
			}
			if seeds > 0 {
				o.Extra /= float64(seeds)
				o.Peak /= float64(seeds)
				o.Took /= time.Duration(seeds)
			}
			out = append(out, o)
		}
	}
	return out
}

// FindIncident returns the incident with the given key.
func FindIncident(key string) (Incident, bool) {
	for _, inc := range Incidents {
		if inc.Key == key {
			return inc, true
		}
	}
	return Incident{}, false
}
