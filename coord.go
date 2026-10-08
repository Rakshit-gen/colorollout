package colorollout

import (
	"fmt"
	"sync"
)

// Coordinator lets each service have one rollout in flight. Two releases of
// the same service overlapping would share the canary scopes, and a revert
// of one could land on top of the other.
type Coordinator struct {
	mu     sync.Mutex
	active map[string]*Rollout
}

// Begin registers a rollout for its plan's service, or says who holds it.
func (c *Coordinator) Begin(r *Rollout) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.active == nil {
		c.active = map[string]*Rollout{}
	}
	svc := r.Plan.Service
	if cur, ok := c.active[svc]; ok && cur.State == Running {
		return fmt.Errorf("%s already has a rollout at stage %d", svc, cur.Stage+1)
	}
	c.active[svc] = r
	return nil
}

// Active returns the service's current rollout, if any is running.
func (c *Coordinator) Active(service string) *Rollout {
	c.mu.Lock()
	defer c.mu.Unlock()
	if r := c.active[service]; r != nil && r.State == Running {
		return r
	}
	return nil
}
