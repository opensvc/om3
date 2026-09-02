// Package metricsreg holds the prometheus registries the daemon serves
// apart from the default one.
//
// A subsystem whose metrics carry a label per object grows its series
// count with the size of the cluster, and the default registry is scraped
// at whatever interval the operator picked for everything else, often a
// few seconds. Putting that detail in its own registry lets it be served
// under its own path and scraped on its own, much longer, interval, or
// not at all until someone is looking.
//
// The split moves work rather than removing it: scraping every endpoint
// at the same interval costs what one endpoint cost, plus the extra
// requests. It pays off only when the detail is scraped less often, which
// is a scrape config decision, not one this package can make.
//
// What stays on the default registry is a handful of aggregates per
// subsystem, enough to tell that drilling down is worth it. The rule they
// were chosen by: if no alert would fire on it, it is not a hint, it is
// noise.
package metricsreg

import "github.com/prometheus/client_golang/prometheus"

var (
	// API holds the per route series of the http listener, the ones the
	// echo middleware breaks down by url, and the rate limiter denials
	// broken down the same way.
	API = prometheus.NewRegistry()

	// PG holds the per cgroup series, one set per object with a pg.
	PG = prometheus.NewRegistry()

	// Pubsub holds the per message kind and per subscription filter
	// series, the filter one growing with the number of distinct filters
	// the daemon's subscribers use.
	Pubsub = prometheus.NewRegistry()

	// Scheduler holds the per object and per schedule entry run counters.
	Scheduler = prometheus.NewRegistry()
)

// Detail maps the name a registry is served under, at /metrics/<name>, to
// the registry itself. The router reads this, so serving a new subsystem
// is a line here rather than a route.
var Detail = map[string]*prometheus.Registry{
	"api":       API,
	"pg":        PG,
	"pubsub":    Pubsub,
	"scheduler": Scheduler,
}
