package colorollout

import "testing"

func TestAdaptiveLimitSparesOnCall(t *testing.T) {
	const batch, capacity, oncall = 2000, 40, 5
	burst := RunBackend(batch, capacity, oncall, nil, 1)
	adaptive := RunBackend(batch, capacity, oncall, NewQueryLimit(4, 200), 1)
	t.Logf("all at once: %+v", burst)
	t.Logf("adaptive:    %+v", adaptive)
	if adaptive.BatchFailures*5 > burst.BatchFailures {
		t.Errorf("adaptive limit failed %d batch queries, burst %d", adaptive.BatchFailures, burst.BatchFailures)
	}
	burstRate := float64(burst.InteractiveFailures) / float64(burst.Interactive)
	adaptiveRate := float64(adaptive.InteractiveFailures) / float64(adaptive.Interactive)
	if adaptiveRate*2 > burstRate {
		t.Errorf("on-call failure rate %.3f with the limit, %.3f without", adaptiveRate, burstRate)
	}
}
