package colorollout

import (
	"math"
	"math/rand/v2"
	"testing"
)

func TestPoissonMeanAndVariance(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	for _, mean := range []float64{0.05, 3, 25, 400, 24000} {
		const n = 20000
		var sum, sq float64
		for i := 0; i < n; i++ {
			x := float64(poisson(rng, mean))
			sum += x
			sq += x * x
		}
		m := sum / n
		v := sq/n - m*m
		if math.Abs(m-mean) > 4*math.Sqrt(mean/n)+0.01 {
			t.Errorf("mean %v: got mean %v", mean, m)
		}
		if math.Abs(v-mean)/mean > 0.06 {
			t.Errorf("mean %v: got variance %v", mean, v)
		}
	}
	if poisson(rng, 0) != 0 || poisson(rng, -1) != 0 {
		t.Fatal("non-positive mean should draw 0")
	}
}
