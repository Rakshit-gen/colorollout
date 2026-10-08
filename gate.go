package colorollout

import "fmt"

// Decision is what a health check says to do with a stage.
type Decision int

const (
	Wait     Decision = iota // not enough evidence either way yet
	Continue                 // healthy for long enough: move on
	Revert                   // the new version is hurting: roll back
)

func (d Decision) String() string {
	return [...]string{"wait", "continue", "revert"}[d]
}

// Gate turns counts into a decision. It reverts when the new version is
// clearly over its SLO, or clearly worse than the old version running beside
// it, and it won't call anything healthy on too little traffic.
type Gate struct {
	MinRequests int64   // requests on the new version before judging at all
	Z           float64 // standard errors needed to call a version bad
	// Confirm is how many checks in a row must say revert before the
	// rollout does. A gate that looks every minute for hours runs hundreds
	// of tests, and at three standard errors some good releases will cross
	// the line once by chance.
	Confirm int
}

// DefaultGate needs a thousand requests, four standard errors, and two
// checks in a row. At three standard errors and one check, the backtest
// reverted 36 of 600 good releases; at these settings it reverted none.
var DefaultGate = Gate{MinRequests: 1000, Z: 4, Confirm: 2}

// Verdict is a decision with the reason behind it.
type Verdict struct {
	Decision Decision
	Reason   string
}

// Judge checks one SLO. canary is the new version, control the old version
// in the same scopes over the same window.
func (g Gate) Judge(slo SLO, canary, control Counts) Verdict {
	if canary.Requests < g.MinRequests {
		return Verdict{Wait, fmt.Sprintf("%s: %d requests, need %d", slo.Name, canary.Requests, g.MinRequests)}
	}
	if z := AboveObjective(canary, slo.Objective); z >= g.Z {
		return Verdict{Revert, fmt.Sprintf("%s: %.3f%% failing, objective %.3f%% (z=%.1f)",
			slo.Name, 100*canary.Ratio(), 100*slo.Objective, z)}
	}
	if z := WorseThan(canary, control); z >= g.Z {
		return Verdict{Revert, fmt.Sprintf("%s: %.3f%% failing vs %.3f%% on the old version (z=%.1f)",
			slo.Name, 100*canary.Ratio(), 100*control.Ratio(), z)}
	}
	return Verdict{Continue, slo.Name + ": healthy"}
}

// JudgeAll checks every SLO. Any revert wins, then any wait; the stage only
// continues when every SLO says so. counts are keyed by SLO name.
func (g Gate) JudgeAll(slos []SLO, canary, control map[string]Counts) Verdict {
	out := Verdict{Continue, "all SLOs healthy"}
	for _, slo := range slos {
		v := g.Judge(slo, canary[slo.Name], control[slo.Name])
		if v.Decision == Revert {
			return v
		}
		if v.Decision == Wait && out.Decision == Continue {
			out = v
		}
	}
	return out
}
