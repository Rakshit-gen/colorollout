package colorollout

import "testing"

func TestParseScope(t *testing.T) {
	sc, err := ParseScope("tier=3 color=green,blue")
	if err != nil {
		t.Fatal(err)
	}
	f := NewFleet([]int{2, 3, 5}, 6, 1)
	got := f.In(sc)
	if len(got) != 5*4 { // 5 tier-3 DCs, 2 of 3 colors of 6 servers
		t.Fatalf("%d servers", len(got))
	}
	for _, s := range got {
		if s.DC.Tier != 3 || s.Color == "red" {
			t.Fatalf("%+v slipped in", s)
		}
	}
	if sc.String() != "tier=3 color=green,blue" {
		t.Fatal(sc.String())
	}
	all, _ := ParseScope("everywhere")
	if len(f.In(all)) != len(f.Servers) {
		t.Fatal("everywhere is not everything")
	}
}

func TestParseScopeErrors(t *testing.T) {
	for _, s := range []string{"tier=0", "tier=x", "color=purple", "planet=mars", "tier"} {
		if _, err := ParseScope(s); err == nil {
			t.Errorf("%q accepted", s)
		}
	}
}
