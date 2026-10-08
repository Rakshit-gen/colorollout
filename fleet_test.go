package colorollout

import "testing"

func TestNewFleetShape(t *testing.T) {
	f := NewFleet([]int{2, 3, 5}, 6, 1)
	if len(f.DCs) != 10 || len(f.Servers) != 60 {
		t.Fatalf("%d dcs, %d servers", len(f.DCs), len(f.Servers))
	}
	colors := map[string]int{}
	for _, s := range f.Servers {
		colors[s.Color]++
		if s.Load <= 0 {
			t.Fatal("server with no load")
		}
	}
	for _, c := range Colors {
		if colors[c] != 20 {
			t.Fatalf("%s: %d servers", c, colors[c])
		}
	}
	g := NewFleet([]int{2, 3, 5}, 6, 1)
	if g.Servers[17].Load != f.Servers[17].Load {
		t.Fatal("same seed, different fleet")
	}
}
