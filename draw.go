package colorollout

import (
	"math"
	"math/rand/v2"
)

// poisson draws a Poisson count with the given mean. Small means use Knuth's
// method; large ones a rounded normal, which is close enough for request
// counts and doesn't loop thousands of times per server per tick.
func poisson(rng *rand.Rand, mean float64) int64 {
	if mean <= 0 {
		return 0
	}
	if mean < 30 {
		limit, k, p := math.Exp(-mean), int64(0), 1.0
		for {
			p *= rng.Float64()
			if p <= limit {
				return k
			}
			k++
		}
	}
	n := math.Round(mean + math.Sqrt(mean)*rng.NormFloat64())
	if n < 0 {
		return 0
	}
	return int64(n)
}
