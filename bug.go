package colorollout

import (
	"fmt"
	"time"
)

// Bug is what a bad release does: the failure ratio the new version has on
// server s for one SLO, given the old version's ratio and how long s has run
// the new version.
type Bug interface {
	Ratio(slo string, s *Server, base float64, running time.Duration) float64
	String() string
}

// NoBug is a good release.
type NoBug struct{}

func (NoBug) Ratio(_ string, _ *Server, base float64, _ time.Duration) float64 { return base }
func (NoBug) String() string                                                   { return "no bug" }

// Raise multiplies one SLO's failure ratio on every server, or only on servers
// in Where: a bug that only shows up on some hardware or in some region.
type Raise struct {
	SLO    string
	Factor float64
	Where  Scope
}

func (b Raise) Ratio(slo string, s *Server, base float64, _ time.Duration) float64 {
	if slo != b.SLO || !b.Where.Match(s) {
		return base
	}
	return min(1, base*b.Factor)
}

func (b Raise) String() string {
	return fmt.Sprintf("%s x%g %s", b.SLO, b.Factor, b.Where)
}

// Crash makes the new version fail a share of every request on matching
// servers, for every SLO: a process that keeps restarting.
type Crash struct {
	Share float64
	Where Scope
}

func (b Crash) Ratio(_ string, s *Server, base float64, _ time.Duration) float64 {
	if !b.Where.Match(s) {
		return base
	}
	return base + (1-base)*b.Share
}

func (b Crash) String() string { return fmt.Sprintf("crash %g%% %s", 100*b.Share, b.Where) }
