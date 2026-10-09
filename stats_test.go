package colorollout

import (
	"math"
	"testing"
)

func TestAboveObjective(t *testing.T) {
	// 0.2% failures against a 0.1% objective over 100k requests:
	// se = sqrt(0.001*0.999/1e5) = 0.0000999, so z is about 10.
	z := AboveObjective(Counts{100000, 200}, 0.001)
	if math.Abs(z-10.005) > 0.01 {
		t.Fatalf("z = %v", z)
	}
	// The same ratio over 1000 requests is weak evidence.
	if z := AboveObjective(Counts{1000, 2}, 0.001); z > 1.1 {
		t.Fatalf("z = %v on 1000 requests", z)
	}
	if AboveObjective(Counts{}, 0.001) != 0 {
		t.Fatal("no requests should give no evidence")
	}
}

func TestWorseThan(t *testing.T) {
	same := WorseThan(Counts{50000, 50}, Counts{500000, 500})
	if math.Abs(same) > 0.01 {
		t.Fatalf("equal ratios: z = %v", same)
	}
	worse := WorseThan(Counts{50000, 100}, Counts{500000, 500})
	if worse < 5 {
		t.Fatalf("double the failures: z = %v", worse)
	}
	if WorseThan(Counts{50000, 25}, Counts{500000, 500}) >= 0 {
		t.Fatal("better canary scored as worse")
	}
	if z := WorseThan(Counts{100, 100}, Counts{1000, 1000}); z != 0 {
		t.Fatalf("both versions failing every request: z = %v", z)
	}
}
