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

// In returns the servers in f that the scope matches, in fleet order.
// Scopes that only name tiers and colors, which is most stages, are read
// from the release scope index instead of scanning every server.
func (f *Fleet) In(sc Scope) []*Server {
	if len(sc.Tiers) > 0 && len(sc.Regions) == 0 && len(sc.DCs) == 0 {
		idx := f.releaseScopes()
		colors := sc.Colors
		if len(colors) == 0 {
			colors = Colors
		}
		var out []*Server
		for _, t := range sc.Tiers {
			for _, c := range colors {
				out = append(out, idx[tierColor{t, c}]...)
			}
		}
		slices.SortFunc(out, func(a, b *Server) int { return a.ID - b.ID })
		return slices.Compact(out) // "tier=3,3" names a tier twice
	}
	var out []*Server
	for _, s := range f.Servers {
		if sc.Match(s) {
			out = append(out, s)
		}
	}
	return out
}

type tierColor struct {
	tier  int
	color string
}

// releaseScopes groups servers by tier and color once, the way HMD
// precomputes hmd:release_scopes:info with a Prometheus recording rule
// instead of joining server and data center metadata on every query.
func (f *Fleet) releaseScopes() map[tierColor][]*Server {
	f.once.Do(func() {
		f.index = map[tierColor][]*Server{}
		for _, s := range f.Servers {
			k := tierColor{s.DC.Tier, s.Color}
			f.index[k] = append(f.index[k], s)
		}
	})
	return f.index
}
