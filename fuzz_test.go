package colorollout

import "testing"

func FuzzParseScope(f *testing.F) {
	for _, s := range []string{"tier=3 color=green", "everywhere", "traffic=10%", "dc=t3-eu-04", "tier=0", "traffic=1e309%", "traffic=NaN%", "color=", "region=a,,b"} {
		f.Add(s)
	}
	fl := NewFleet([]int{1, 1, 1}, 3, 1)
	f.Fuzz(func(t *testing.T, s string) {
		sc, err := ParseScope(s)
		if err != nil {
			return
		}
		if sh := sc.Share(); !(sh > 0 && sh <= 1) {
			t.Fatalf("%q: share %v", s, sh)
		}
		fl.In(sc)
		back, err := ParseScope(sc.String())
		if err != nil || back.String() != sc.String() {
			t.Fatalf("%q prints as %q, which reads back as %q (%v)", s, sc, back, err)
		}
	})
}

func FuzzParsePlan(f *testing.F) {
	f.Add([]byte(`{"service":"x","stages":[{"name":"a","scope":"everywhere","soak":"1m"}],"slos":[{"name":"5xx","objective":0.001}]}`))
	f.Add([]byte(`{"service":"x","window":"-1s"}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		p, err := ParsePlan(b)
		if err == nil && p.Validate() != nil {
			t.Fatal("ParsePlan returned a plan that doesn't validate")
		}
	})
}
