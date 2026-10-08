package colorollout

import "math"

// Counts is requests and failures seen on one version in some window.
type Counts struct {
	Requests int64 `json:"requests"`
	Failures int64 `json:"failures"`
}

// Add returns the sum of two counts.
func (c Counts) Add(o Counts) Counts {
	return Counts{c.Requests + o.Requests, c.Failures + o.Failures}
}

// Ratio is the failure ratio, 0 with no requests.
func (c Counts) Ratio() float64 {
	if c.Requests == 0 {
		return 0
	}
	return float64(c.Failures) / float64(c.Requests)
}

// AboveObjective returns how many standard errors the observed failure ratio
// sits above objective, treating failures as binomial. Large positive values
// mean the version is breaking the SLO, not just unlucky.
func AboveObjective(c Counts, objective float64) float64 {
	if c.Requests == 0 || objective <= 0 || objective >= 1 {
		return 0
	}
	se := math.Sqrt(objective * (1 - objective) / float64(c.Requests))
	return (c.Ratio() - objective) / se
}

// WorseThan returns the two-proportion z score of canary's failure ratio
// against control's: how many standard errors worse the new version is than
// the old one running in the same scopes at the same time. Comparing with a
// control catches a change that makes things worse even while both stay
// inside the SLO, and doesn't blame the release for a bad hour everywhere.
func WorseThan(canary, control Counts) float64 {
	if canary.Requests == 0 || control.Requests == 0 {
		return 0
	}
	pooled := float64(canary.Failures+control.Failures) / float64(canary.Requests+control.Requests)
	if pooled == 0 {
		return 0
	}
	se := math.Sqrt(pooled * (1 - pooled) * (1/float64(canary.Requests) + 1/float64(control.Requests)))
	return (canary.Ratio() - control.Ratio()) / se
}
