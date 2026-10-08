package colorollout

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Scope selects servers. An empty field matches everything, so the zero Scope
// is the whole fleet.
type Scope struct {
	Tiers   []int
	Colors  []string
	Regions []string
	DCs     []string // single data centers by name, for a first canary
}

// Match reports whether s is in the scope.
func (sc Scope) Match(s *Server) bool {
	return (len(sc.Tiers) == 0 || slices.Contains(sc.Tiers, s.DC.Tier)) &&
		(len(sc.Colors) == 0 || slices.Contains(sc.Colors, s.Color)) &&
		(len(sc.Regions) == 0 || slices.Contains(sc.Regions, s.DC.Region)) &&
		(len(sc.DCs) == 0 || slices.Contains(sc.DCs, s.DC.Name))
}

func (sc Scope) String() string {
	var parts []string
	add := func(k string, vs []string) {
		if len(vs) > 0 {
			parts = append(parts, k+"="+strings.Join(vs, ","))
		}
	}
	var tiers []string
	for _, t := range sc.Tiers {
		tiers = append(tiers, strconv.Itoa(t))
	}
	add("dc", sc.DCs)
	add("tier", tiers)
	add("color", sc.Colors)
	add("region", sc.Regions)
	if len(parts) == 0 {
		return "everywhere"
	}
	return strings.Join(parts, " ")
}

// ParseScope reads a scope such as "tier=3 color=green,blue". "everywhere"
// and the empty string are the whole fleet.
func ParseScope(s string) (Scope, error) {
	var sc Scope
	s = strings.TrimSpace(s)
	if s == "" || s == "everywhere" {
		return sc, nil
	}
	for _, f := range strings.Fields(s) {
		k, v, ok := strings.Cut(f, "=")
		if !ok || v == "" {
			return sc, fmt.Errorf("scope %q: want key=value, got %q", s, f)
		}
		vals := strings.Split(v, ",")
		switch k {
		case "tier":
			for _, x := range vals {
				n, err := strconv.Atoi(x)
				if err != nil || n < 1 {
					return sc, fmt.Errorf("scope %q: bad tier %q", s, x)
				}
				sc.Tiers = append(sc.Tiers, n)
			}
		case "color":
			for _, c := range vals {
				if !slices.Contains(Colors, c) {
					return sc, fmt.Errorf("scope %q: unknown color %q", s, c)
				}
			}
			sc.Colors = vals
		case "region":
			sc.Regions = vals
		case "dc":
			sc.DCs = vals
		default:
			return sc, fmt.Errorf("scope %q: unknown key %q", s, k)
		}
	}
	return sc, nil
}

// In returns the servers in f that the scope matches.
func (f *Fleet) In(sc Scope) []*Server {
	var out []*Server
	for _, s := range f.Servers {
		if sc.Match(s) {
			out = append(out, s)
		}
	}
	return out
}
