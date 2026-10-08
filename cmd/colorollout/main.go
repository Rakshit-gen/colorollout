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
	case "backtest":
		backtest(os.Args[2:])
	case "queries":
		queries(os.Args[2:])
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: colorollout validate [flags] PLAN...")
	fmt.Fprintln(os.Stderr, "       colorollout run [flags] PLAN")
	fmt.Fprintln(os.Stderr, "       colorollout backtest [flags] PLAN...")
	fmt.Fprintln(os.Stderr, "       colorollout queries [flags]")
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
	journal := fs.String("journal", "", "keep the rollout's journal here and resume from it if it exists")
	stop := fs.Duration("stop-after", 48*time.Hour, "stop this long into the release, as if the process died")
	freeze := fs.String("freeze", "", "a change freeze as FROM-TO into the release, such as 20m-2h")
	fs.Parse(args)
	if fs.NArg() != 1 {
		usage()
	}
	p := loadPlan(fs.Arg(0))
	i := incident(*inc)
	f := fleet()
	w := colorollout.NewWorld(f, baseRatios(p, *base), i.Bug, *seed)
	r := colorollout.NewRollout(p, colorollout.DefaultGate, f)
	r.Page = func(e colorollout.Event) { fmt.Printf("PAGE %s: %s\n", p.Service, e.What) }
	shown := 0
	if *journal != "" {
		j, events, err := colorollout.OpenJournal(*journal)
		if err != nil {
			fatalf("%v", err)
		}
		defer j.Close()
		if len(events) > 0 {
			w.Skip(events[len(events)-1].At)
			r = colorollout.Resume(p, colorollout.DefaultGate, f, events, w.Now())
			shown = len(events)
			r.Page = func(e colorollout.Event) { fmt.Printf("PAGE %s: %s\n", p.Service, e.What) }
			fmt.Printf("resuming from %s: stage %d, %v\n", *journal, r.Stage+1, r.State)
		}
		r.Journal = j
	}
	if r.State != colorollout.Running {
		return
	}
	if *freeze != "" {
		from, to, ok := strings.Cut(*freeze, "-")
		a, err1 := time.ParseDuration(from)
		b, err2 := time.ParseDuration(to)
		if !ok || err1 != nil || err2 != nil || b <= a {
			fatalf("-freeze: want FROM-TO such as 20m-2h, got %q", *freeze)
		}
		// Event times count the baseline; the flag counts from the release.
		r.Freezes = []colorollout.Freeze{{From: a + colorollout.Baseline, To: b + colorollout.Baseline}}
	}
	res := colorollout.Drive(r, w, time.Minute, *stop)
	fmt.Printf("%s, release with %s\n\n", p.Service, i.Name)
	for _, e := range res.Events[shown:] {
		if e.Kind != "baseline" {
			e.At -= colorollout.Baseline // from the start of the release
			fmt.Println(e)
		}
	}
	if res.State == colorollout.Running {
		fmt.Printf("\nstopped after %v, still at stage %d\n", res.Took, r.Stage+1)
		return
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

func backtest(args []string) {
	fs := flag.NewFlagSet("backtest", flag.ExitOnError)
	fleet := fleetFlags(fs)
	seeds := fs.Int("seeds", 20, "runs per plan and incident")
	base := fs.Float64("base", 0.0003, "the old version's failure ratio")
	fs.Parse(args)
	if fs.NArg() == 0 {
		usage()
	}
	var plans []colorollout.Plan
	for _, path := range fs.Args() {
		plans = append(plans, loadPlan(path))
	}
	f := fleet()
	fmt.Printf("%-14s %-26s %7s %9s %10s %8s\n", "plan", "release", "caught", "detect", "extra", "reached")
	for _, o := range colorollout.Backtest(plans, colorollout.Incidents, f, baseRatios(plans[0], *base), *seeds) {
		detect := "-"
		if o.Caught > 0 {
			detect = o.Detect.Round(time.Second).String()
		}
		fmt.Printf("%-14s %-26s %3d/%-3d %9s %10.0f %7.1f%%\n",
			o.Plan, o.Incident, o.Caught, o.Runs, detect, o.Extra, 100*o.Peak)
	}
}

func queries(args []string) {
	fs := flag.NewFlagSet("queries", flag.ExitOnError)
	batch := fs.Int("batch", 2000, "health queries in one batch")
	capacity := fs.Int("capacity", 40, "queries the backend serves per tick")
	oncall := fs.Int("oncall", 5, "mean on-call queries per tick")
	minLimit := fs.Float64("min", 4, "floor for the adaptive limit")
	maxLimit := fs.Float64("max", 200, "ceiling for the adaptive limit")
	seed := fs.Uint64("seed", 1, "random seed")
	fs.Parse(args)
	fmt.Printf("%-12s %6s %15s %16s\n", "", "ticks", "batch failures", "on-call failing")
	show := func(name string, r colorollout.BackendRun) {
		fmt.Printf("%-12s %6d %15d %15.1f%%\n", name, r.Ticks, r.BatchFailures,
			100*float64(r.InteractiveFailures)/float64(max(1, r.Interactive)))
	}
	show("all at once", colorollout.RunBackend(*batch, *capacity, *oncall, nil, *seed))
	fixed := colorollout.NewQueryLimit(float64(*capacity), float64(*capacity))
	show("fixed limit", colorollout.RunBackend(*batch, *capacity, *oncall, fixed, *seed))
	show("adaptive", colorollout.RunBackend(*batch, *capacity, *oncall, colorollout.NewQueryLimit(*minLimit, *maxLimit), *seed))
}
