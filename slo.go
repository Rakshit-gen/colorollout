package colorollout

// SLO is a health objective: the share of failed requests on the new version
// must stay under Objective. Cloudflare's example is 500 errors under 0.1%
// of requests over ten minutes.
type SLO struct {
	Name      string
	Objective float64 // highest acceptable failure ratio, e.g. 0.001
}
