package metricsplit

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	// Imported for their metric registrations, which happen in init.
	_ "github.com/opensvc/om3/v3/daemon/scheduler"
	_ "github.com/opensvc/om3/v3/util/pubsub"

	"github.com/opensvc/om3/v3/util/metricsreg"
)

// isRegistered reports whether a metric name is taken in reg.
//
// Gather is no help here: it only returns the families that have at least
// one series, and a vector has none until a label combination is used, so
// a registered but untouched metric looks absent. Registering a probe of
// the same name is the question the registry can answer either way.
func isRegistered(reg prometheus.Registerer, name string) bool {
	probe := prometheus.NewGauge(prometheus.GaugeOpts{Name: name, Help: "probe"})
	if err := reg.Register(probe); err != nil {
		return true
	}
	reg.Unregister(probe)
	return false
}

func names(t *testing.T, g prometheus.Gatherer) map[string]bool {
	t.Helper()
	families, err := g.Gather()
	require.NoError(t, err)
	m := make(map[string]bool, len(families))
	for _, f := range families {
		m[f.GetName()] = true
	}
	return m
}

// TestNoMetricNameIsServedTwice is the hazard of splitting registries: a
// name in both the default registry and a detail one is scraped twice
// into the same prometheus, and any sum over it silently double counts.
func TestNoMetricNameIsServedTwice(t *testing.T) {
	for endpoint, registry := range metricsreg.Detail {
		for name := range names(t, registry) {
			assert.False(t, isRegistered(prometheus.DefaultRegisterer, name),
				"%s is served by both /metrics and /metrics/%s", name, endpoint)
		}
		for name := range names(t, prometheus.DefaultGatherer) {
			assert.False(t, isRegistered(registry, name),
				"%s is served by both /metrics and /metrics/%s", name, endpoint)
		}
	}
}

// TestDetailIsNotOnTheDefaultRegistry pins where the per object series
// went. These are the names the split exists for.
func TestDetailIsNotOnTheDefaultRegistry(t *testing.T) {
	for endpoint, want := range map[string][]string{
		"pubsub": {
			"opensvc_pubsub_publication_pushed_by_filter_total",
			"opensvc_pubsub_publication_by_kind_total",
			"opensvc_pubsub_subscription_filter_by_kind_total",
		},
		"scheduler": {
			"opensvc_scheduler_object_runs_total",
			"opensvc_scheduler_object_job_runs_total",
		},
	} {
		for _, name := range want {
			assert.True(t, isRegistered(metricsreg.Detail[endpoint], name), "%s must be served at /metrics/%s", name, endpoint)
			assert.False(t, isRegistered(prometheus.DefaultRegisterer, name), "%s must not be on /metrics", name)
		}
	}
}

// TestHintsAreOnTheDefaultRegistry pins the other half: the aggregates
// have to stay where the normal scrape finds them, or the split has
// hidden the subsystem instead of summarising it.
func TestHintsAreOnTheDefaultRegistry(t *testing.T) {
	for _, name := range []string{
		"opensvc_pubsub_publication_total",
		"opensvc_pubsub_publication_pushed_total",
		"opensvc_pubsub_subscription_filter_total",
		"opensvc_pubsub_filterkeys",
		"opensvc_pubsub_subscription_total",
		"opensvc_pubsub_subscription_queue_full_total",
		"opensvc_pubsub_subscription_queue_threshold_total",
		"opensvc_scheduler_runs_total",
	} {
		assert.True(t, isRegistered(prometheus.DefaultRegisterer, name), "%s must stay on /metrics as a drill down hint", name)
	}
}
