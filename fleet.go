// Package colorollout releases a change across a fleet of data centers in
// stages, watching service health at each step and rolling back on its own
// when the change makes things worse. It follows what Cloudflare has written
// about Health Mediated Deployments: the fleet is sliced into release scopes
// by tier and color, each service defines its own health signals and plan,
// and every step either continues, waits for more evidence, or reverts.
package colorollout

import (
	"fmt"
	"math/rand/v2"
)

// Colors split the servers inside every data center into groups that can
// run different versions side by side.
var Colors = []string{"red", "green", "blue"}

// DataCenter is one location. Tier 1 is the largest; higher tiers are
// smaller sites, which is where a release usually starts.
type DataCenter struct {
	Name   string
	Tier   int
	Region string
}

// Server is one machine. Load is its share of its data center's traffic.
type Server struct {
	ID    int
	DC    *DataCenter
	Color string
	Load  float64 // requests per second
}

// Fleet is every server in every data center.
type Fleet struct {
	DCs     []*DataCenter
	Servers []*Server
}

// Regions the generated fleet spreads over.
var Regions = []string{"na", "eu", "apac", "latam", "africa"}

// NewFleet builds a repeatable fleet: tierCounts[i] data centers of tier i+1,
// each with perDC servers. Bigger tiers get more traffic per server.
func NewFleet(tierCounts []int, perDC int, seed uint64) *Fleet {
	rng := rand.New(rand.NewPCG(seed, 7))
	f := &Fleet{}
	id := 0
	for t, n := range tierCounts {
		tier := t + 1
		for i := 0; i < n; i++ {
			dc := &DataCenter{
				Name:   fmt.Sprintf("t%d-%s-%02d", tier, Regions[(i+t)%len(Regions)], i),
				Tier:   tier,
				Region: Regions[(i+t)%len(Regions)],
			}
			f.DCs = append(f.DCs, dc)
			for s := 0; s < perDC; s++ {
				base := 400.0 / float64(tier) // rps per server
				f.Servers = append(f.Servers, &Server{
					ID:    id,
					DC:    dc,
					Color: Colors[s%len(Colors)],
					Load:  base * (0.7 + 0.6*rng.Float64()),
				})
				id++
			}
		}
	}
	return f
}
