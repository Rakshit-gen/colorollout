package colorollout

import "testing"

func TestOneRolloutPerService(t *testing.T) {
	var c Coordinator
	f := NewFleet([]int{1, 1, 1}, 3, 1)
	a := NewRollout(testPlan(t), testGate, f)
	if err := c.Begin(a); err != nil {
		t.Fatal(err)
	}
	b := NewRollout(testPlan(t), testGate, f)
	if err := c.Begin(b); err == nil {
		t.Fatal("second rollout of the same service allowed")
	}
	other := testPlan(t)
	other.Service = "dns"
	if err := c.Begin(NewRollout(other, testGate, f)); err != nil {
		t.Fatalf("different service blocked: %v", err)
	}
	a.State = RolledBack
	if c.Active("edge") != nil {
		t.Fatal("finished rollout still active")
	}
	if err := c.Begin(b); err != nil {
		t.Fatalf("blocked after the first one ended: %v", err)
	}
}
