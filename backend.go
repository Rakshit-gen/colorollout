package colorollout

import "math/rand/v2"

// BackendRun is how a batch of health queries went against a simulated
// metrics backend.
type BackendRun struct {
	Ticks               int // until every batch query succeeded
	BatchFailures       int // failed batch queries, each retried
	Interactive         int // on-call queries sent meanwhile
	InteractiveFailures int
}

// RunBackend sends batch health queries to a backend that can serve
// capacity queries per tick. Each tick on-call engineers also send 0 to
// 2*oncall queries, which skip any limit. When more than capacity queries
// arrive in a tick, each fails with the share that is over. Failed batch
// queries go back in the queue.
//
// limit decides how many batch queries go out per tick: nil sends all of
// them at once.
func RunBackend(batch, capacity, oncall int, limit *QueryLimit, seed uint64) BackendRun {
	rng := rand.New(rand.NewPCG(seed, 5))
	var run BackendRun
	pending := batch
	for pending > 0 && run.Ticks < 100000 {
		run.Ticks++
		inter := rng.IntN(2*oncall + 1)
		sent := pending
		if limit != nil {
			sent = 0
			for sent < pending && limit.TryAcquire() {
				sent++
			}
		}
		total := sent + inter
		fail := 0.0
		if total > capacity {
			fail = float64(total-capacity) / float64(total)
		}
		for i := 0; i < sent; i++ {
			ok := rng.Float64() >= fail
			if ok {
				pending--
			} else {
				run.BatchFailures++
			}
			if limit != nil {
				limit.Release(ok)
			}
		}
		run.Interactive += inter
		for i := 0; i < inter; i++ {
			if rng.Float64() < fail {
				run.InteractiveFailures++
			}
		}
	}
	return run
}
