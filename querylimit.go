package colorollout

import (
	"context"
	"sync"
)

// QueryLimit caps how many health queries run against the metrics backend
// at once and moves the cap with how the backend is doing. Every success
// lets one more query through next time; every failure halves the cap, but
// never below Min, so a backend that's struggling still gets enough queries
// to keep rollouts judged. Cloudflare describes doing this for HMD's batch
// queries to Thanos, after Netflix's adaptive concurrency limits.
//
// Interactive queries, the ones an on-call engineer runs while triaging,
// skip the limit: the batch work is what should back off.
type QueryLimit struct {
	Min, Max float64

	mu       sync.Mutex
	limit    float64
	inflight int
	wake     chan struct{}
}

// NewQueryLimit starts at min.
func NewQueryLimit(min, max float64) *QueryLimit {
	return &QueryLimit{Min: min, Max: max, limit: min, wake: make(chan struct{})}
}

// Limit is the current cap.
func (q *QueryLimit) Limit() float64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.limit
}

// Acquire waits for room for one batch query.
func (q *QueryLimit) Acquire(ctx context.Context) error {
	for {
		q.mu.Lock()
		if float64(q.inflight) < q.limit {
			q.inflight++
			q.mu.Unlock()
			return nil
		}
		wake := q.wake
		q.mu.Unlock()
		select {
		case <-wake:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Release ends a query and adjusts the cap by how it went.
func (q *QueryLimit) Release(ok bool) {
	q.mu.Lock()
	q.inflight--
	if ok {
		q.limit = min(q.Max, q.limit+1)
	} else {
		q.limit = max(q.Min, q.limit/2)
	}
	close(q.wake)
	q.wake = make(chan struct{})
	q.mu.Unlock()
}
