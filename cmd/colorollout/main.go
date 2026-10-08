// Command colorollout checks rollout plans and replays them against a
// simulated fleet.
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Rakshit-gen/colorollout"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "validate":
		validate(os.Args[2:])
	case "run":
		run(os.Args[2:])
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: colorollout validate [flags] PLAN...")
	fmt.Fprintln(os.Stderr, "       colorollout run [flags] PLAN")
	os.Exit(2)
}

// fleetFlags adds the flags that describe the simulated fleet.
func fleetFlags(fs *flag.FlagSet) func() *colorollout.Fleet {
	tiers := fs.String("tiers", "10,20,30", "data centers in tier 1, tier 2, ...")
	perDC := fs.Int("per-dc", 12, "servers per data center")
	seed := fs.Uint64("fleet-seed", 1, "seed for server loads")
	return func() *colorollout.Fleet {
		var counts []int
		for _, s := range strings.Split(*tiers, ",") {
			n, err := strconv.Atoi(s)
			if err != nil || n < 0 {
				fatalf("-tiers: bad count %q", s)
			}
			counts = append(counts, n)
		}
		return colorollout.NewFleet(counts, *perDC, *seed)
	}
}

func loadPlan(path string) colorollout.Plan {
	b, err := os.ReadFile(path)
	if err != nil {
		fatalf("%v", err)
	}
	p, err := colorollout.ParsePlan(b)
	if err != nil {
		fatalf("%s: %v", path, err)
	}
	return p
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "colorollout: "+format+"\n", args...)
	os.Exit(1)
}

func validate(args []string) {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	fleet := fleetFlags(fs)
	fs.Parse(args)
	if fs.NArg() == 0 {
		usage()
	}
	f := fleet()
	var total float64
	for _, s := range f.Servers {
		total += s.Load
	}
	for _, path := range fs.Args() {
		p := loadPlan(path)
		fmt.Printf("%s: %s, %d stages\n", path, p.Service, len(p.Stages))
		var soak float64
		for i, st := range p.Stages {
			var load float64
			servers := f.In(st.Scope)
			for _, s := range servers {
				load += s.Load
			}
			soak += st.Soak.Minutes()
			fmt.Printf("  %d  %-36s %5d servers  %6.2f%% of traffic  soak %v\n",
				i+1, st.Scope, len(servers), 100*load*st.Scope.Share()/total, st.Soak)
			if len(servers) == 0 {
				fatalf("%s: stage %d matches no servers", path, i+1)
			}
		}
		fmt.Printf("  at least %.0f minutes if every stage passes first time\n", soak)
	}
}

func incident(key string) colorollout.Incident {
	inc, ok := colorollout.FindIncident(key)
	if !ok {
		var keys []string
		for _, i := range colorollout.Incidents {
			keys = append(keys, i.Key)
		}
		fatalf("unknown incident %q; have %s", key, strings.Join(keys, ", "))
	}
	return inc
}

func run(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	fleet := fleetFlags(fs)
	inc := fs.String("incident", "good", "what the release does: x10, x3, x1.5, tier1, crash, slowburn, load or good")
	base := fs.Float64("base", 0.0003, "the old version's failure ratio")
	seed := fs.Uint64("seed", 1, "traffic seed")
	fs.Parse(args)
	if fs.NArg() != 1 {
		usage()
	}
	p := loadPlan(fs.Arg(0))
	i := incident(*inc)
	w := colorollout.NewWorld(fleet(), baseRatios(p, *base), i.Bug, *seed)
	res := colorollout.Simulate(p, colorollout.DefaultGate, w, time.Minute, 48*time.Hour)
	fmt.Printf("%s, release with %s\n\n", p.Service, i.Name)
	for _, e := range res.Events {
		if e.Kind != "baseline" {
			e.At -= colorollout.Baseline // from the start of the release
			fmt.Println(e)
		}
	}
	fmt.Printf("\n%s after %v; up to %.1f%% of traffic got it; %.0f extra failed requests\n",
		res.State, res.Took, 100*res.Peak, res.Extra)
}

// baseRatios gives every SLO in the plan the same old-version ratio.
func baseRatios(p colorollout.Plan, ratio float64) map[string]float64 {
	m := map[string]float64{}
	for _, s := range p.SLOs {
		m[s.Name] = ratio
	}
	return m
}
