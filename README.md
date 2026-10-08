# colorollout

Staged rollouts that watch service health at each step and roll back on their own, plus a simulator and backtester
to see what a plan would have done with a bad release.

It is modeled on what Cloudflare has written about Health Mediated Deployments (HMD): the fleet is sliced into
release scopes by data center tier and server color, each service brings its own SLOs and plan, and at every step
the release either continues, waits for more evidence, or reverts. Their Code Orange post describes population
stages for software releases (employee traffic first, then growing shares of customers, starting with free users)
and says configuration changes should go through the same process as code. Not affiliated with Cloudflare. Sources are at the bottom.

```
$ colorollout run -incident x3 examples/edge-proxy.json
edge-proxy, release with errors x3 everywhere

      0s  stage 1  deploy   deploy to dc=t3-eu-04 color=green (4 servers), soak 10m0s
    2m0s  stage 1  revert   5xx: 0.086% failing vs 0.030% on the old version (z=7.9)

rolled back after 2m0s; up to 0.3% of traffic got it; 36 extra failed requests
```

That release kept errors under the 0.1% SLO. It was caught because the gate also compares the new version with
the old one running beside it.

## Install

Go 1.25 or newer.

```sh
git clone https://github.com/Rakshit-gen/colorollout && cd colorollout
go build ./cmd/colorollout
```

## Plans

A plan is JSON. Stages add scopes on top of earlier ones, and the last must cover everything:

```json
{
  "service": "edge-proxy",
  "max_wait": "1h",
  "window": "10m",
  "stages": [
    {"name": "one small site", "scope": "dc=t3-eu-04 color=green", "soak": "10m"},
    {"name": "tier 3, one color", "scope": "tier=3 color=green", "soak": "20m"},
    {"name": "tier 3", "scope": "tier=3", "soak": "20m"},
    {"name": "tier 2", "scope": "tier=2,3", "soak": "30m"},
    {"name": "everywhere", "scope": "everywhere", "soak": "30m"}
  ],
  "slos": [{"name": "5xx", "objective": 0.001}]
}
```

Scopes combine `tier=`, `color=` (red, green, blue), `region=`, `dc=` and `traffic=` (`employees`, `free`, or a
percentage like `10%`). `window` is how far back the gate looks, like `rate(...[10m])`; without it, it looks back to
the start of the stage. `max_wait` is how long a stage may go without enough traffic to judge before it stops and
asks for a person. Unknown fields are an error, so a typo can't quietly drop a safeguard.

`colorollout validate` shows what each stage reaches on the simulated fleet:

```
examples/edge-proxy.json: edge-proxy, 5 stages
  1  dc=t3-eu-04 color=green                  4 servers    0.35% of traffic  soak 10m0s
  2  tier=3 color=green                     120 servers   11.10% of traffic  soak 20m0s
  3  tier=3                                 360 servers   33.53% of traffic  soak 20m0s
  4  tier=2,3                               600 servers   66.58% of traffic  soak 30m0s
  5  everywhere                             720 servers  100.00% of traffic  soak 30m0s
```

## How a stage is judged

Every minute the gate counts requests and failures on the new version over the window, and on the old version over
the same time. It reverts when either:

- the new version's failure ratio is over the SLO by at least 4 standard errors, or
- it's worse than the old version by at least 4 standard errors (a two-proportion z-test),

and that holds for 2 checks in a row. It waits while the new version has under 1,000 requests, and moves on once
every SLO is healthy and the soak time has passed. When there's no old-version traffic left to compare with, as in
the last stage, it compares with 30 minutes of baseline taken before the release started.

Those thresholds came from the backtester. At 3 standard errors and a single check, the gate reverted 36 of 600 good
releases: it checks every minute for hours, and chance crossings add up. Requiring two checks in a row alone still
reverted 19, because overlapping windows make consecutive checks nearly the same test. 4 standard errors twice
reverted none of 600, and caught errors at 1.5 times normal in every run, about 4 minutes later than before.

## Backtest

`colorollout backtest` replays made-up incidents against plans, 20 runs each, on a fleet of 60 data centers (10, 20
and 30 in tiers 1 to 3) with 12 servers each and a 0.03% normal error rate. "Extra" is failed requests the release
caused; "reached" is the largest share of traffic that got it.

| Release | big bang | edge-proxy (by tier) | by-population | mixed |
| --- | --- | --- | --- | --- |
| errors x10 | 2m, 46,601 extra | 2m, 163 extra, 0.4% | 2m, 93, 0.2% | 2m, 31, 0.1% |
| errors x1.5 | 2m, 2,589, 100% | 8m42s, 123, 3.6% | 15m45s, 298, 10.1% | 20m27s, 176, 5.6% |
| errors x10, tier 1 only | 2m, 15,576, 100% | 1h22m, 15,575, 100% | 2m3s, 32, 0.2% | 12m9s, 34, 0.2% |
| crash 20% | 2m, 3,451,262 | 2m, 12,076 | 2m, 6,906 | 2m, 2,319 |
| slow burn after 40m | missed | 59m27s, 1,294, 66.6% | 44m18s, 932, 50% | 47m6s, 478, 25% |
| good release | 0 of 20 reverted | 0 of 20 | 0 of 20 | 0 of 20 |

What it shows:

- Staging by tier alone never touches tier 1 until the last stage, so a bug that only shows on big sites (or only
  above 300 rps per server) does as much harm as a big-bang release, just 80 minutes later. Population stages put
  every site in the first step and catch it at 0.2%. `mixed` does both: employees, then free users, small sites
  first each time.
- Small bugs take longer to find at small scale. Errors at 1.5 times normal need about 15 to 20 minutes on 0.2% of
  traffic to clear 4 standard errors.
- Short soaks miss slow problems. Big bang and `config-push` (5 to 10 minute soaks, for configuration changes) finish
  before a leak that starts at 40 minutes shows up.

All of the incidents are synthetic. They're shaped like the kinds of failures staged rollouts exist for, not taken
from real incident data.

## Crashes and freezes

The rollout writes each event to a journal and syncs it before acting. If the process dies, the next one reads the
journal, puts the fleet back the way it says, and starts the current stage's soak again, because the counts behind
it were in memory. A crash costs time, never a check. A line cut short by a crash is dropped; a bad line anywhere
else is an error. If the journal can't be written, the rollout halts.

```
$ colorollout run -journal edge.journal -stop-after 45m examples/edge-proxy.json
...
stopped after 45m0s, still at stage 3
$ colorollout run -journal edge.journal examples/edge-proxy.json
resuming from edge.journal: stage 3, running
   30m0s  stage 3  resume   picked up from the journal; soak starts again
   50m0s  stage 3  healthy  healthy after 20m0s
```

During a change freeze (`-freeze 20m-2h`) the rollout holds where it is, keeps watching, and can still revert. With
the slow burn incident, a freeze from 20 minutes to 2 hours left it at stage 2, where it was caught at 11% of
traffic instead of 67%. `Coordinator` lets each service have one rollout in flight.

## Health queries

HMD's health checks are batches of metric queries, and Cloudflare describes limiting them with an adaptive
concurrency limit so they back off when the metrics backend struggles, without crowding out on-call engineers'
queries. `QueryLimit` does that: every success lets one more batch query through, every failure halves the limit,
and it never goes below a floor. On-call queries skip it.

`colorollout queries` sends 2,000 batch queries to a simulated backend that serves 40 per tick, with on-call queries
arriving alongside:

| | ticks | batch failures | on-call queries failing |
| --- | --- | --- | --- |
| all at once | 51 | 48,604 | 93.0% |
| fixed limit of 40 | 56 | 230 | 14.3% |
| adaptive, 4 to 200 | 97 | 454 | 7.5% |

The fixed limit only does that well because it was set to the backend's real capacity; the adaptive one finds its
own level. It's slower, which is the point: batch work is what should wait.

## Speed

On an M-series Mac: one simulated minute of 720 servers takes 130 µs, a gate check 380 ns, and a whole rollout
with its baseline 19 ms. The five-plan backtest above runs in about 7 seconds. Tests run in under a second.

## Limits

- Traffic, failures and incidents are all simulated. Requests are Poisson per server per minute, and failures are
  independent, so there's no correlation between servers in the same data center, and no traffic that moves
  between data centers when one goes bad.
- The 4 standard errors and 2 checks were tuned on this simulator's noise. Real metrics are lumpier, and need their
  own backtest against real incident history, which is what Cloudflare describes doing.
- The control group is all old-version traffic, not old-version servers in the same data centers. While the new
  version runs mostly in one tier or region, a difference between places can look like a difference between
  versions.
- Population shares (0.2% employees, 25% including free users) are made up.

## How it is laid out

`fleet.go` and `scope.go` are data centers, servers and release scopes, `plan.go` reads plans, `stats.go` and
`gate.go` make the call, `rollout.go` is the state machine, `journal.go` keeps it across crashes, `coord.go` allows
one rollout per service, `world.go` and `bug.go` simulate traffic and bad releases, `simulate.go` and `backtest.go`
run them, and `querylimit.go` and `backend.go` are the health query limit.

## Sources

- [Scaling with safety: Cloudflare's approach to global service health metrics and software releases](https://blog.cloudflare.com/safe-change-at-any-scale/)
- [Code Orange: Fail Small](https://blog.cloudflare.com/fail-small-resilience-plan/)

## License

MIT
