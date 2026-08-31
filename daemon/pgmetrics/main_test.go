package pgmetrics

import (
	"math"
	"testing"

	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countSeries counts the label combinations a vector currently holds.
// Done by hand rather than with prometheus' testutil, which is not in the
// module graph and would cost a go mod tidy to add.
func countSeries(c prometheus.Collector) int {
	ch := make(chan prometheus.Metric, 1024)
	c.Collect(ch)
	close(ch)
	n := 0
	for range ch {
		n++
	}
	return n
}

func TestParseLimit(t *testing.T) {
	// "max" is what a cgroup v2 limit file holds when nothing is limited,
	// which is the default, so getting this wrong left the metric empty
	// on every cgroup rather than on an unusual one.
	v, err := parseLimit("max\n")
	require.NoError(t, err)
	assert.True(t, math.IsInf(v, 1), "unlimited must be +Inf, got %v", v)

	v, err = parseLimit(" 1073741824 \n")
	require.NoError(t, err)
	assert.Equal(t, float64(1073741824), v)

	_, err = parseLimit("not a number")
	assert.Error(t, err)
}

func TestForgetUsageDropsEverySeriesOfOneCgroupOnly(t *testing.T) {
	gone := cgroupKey{namespace: "ns1", path: "ns1/svc/gone"}
	kept := cgroupKey{namespace: "ns2", path: "ns2/svc/kept"}
	for _, k := range []cgroupKey{gone, kept} {
		pgCgroupMemoryCurrent.WithLabelValues(k.namespace, k.path).Set(1)
		pgCgroupCPUUsage.WithLabelValues(k.namespace, k.path).Set(1)
		// a vec with a third label, to check the partial match reaches it
		pgCgroupMemoryStat.WithLabelValues(k.namespace, k.path, "anon").Set(1)
		pgCgroupMemoryStat.WithLabelValues(k.namespace, k.path, "file").Set(1)
		pgCgroupExists.WithLabelValues(k.namespace, k.path).Set(1)
	}
	t.Cleanup(func() {
		forgetUsage(kept)
		pgCgroupExists.DeletePartialMatch(kept.labels())
		pgCgroupExists.DeletePartialMatch(gone.labels())
	})

	forgetUsage(gone)

	assert.Equal(t, 1, countSeries(pgCgroupMemoryCurrent))
	assert.Equal(t, 1, countSeries(pgCgroupCPUUsage))
	assert.Equal(t, 2, countSeries(pgCgroupMemoryStat), "both stat series of the kept cgroup, neither of the gone one")

	// exists is deliberately left behind by forgetUsage: it is the series
	// that reports the cgroup is not there.
	assert.Equal(t, 2, countSeries(pgCgroupExists))
}

// TestCgroupMetricsCoversTheUsageMetrics guards the enumeration: register,
// unregister and forgetUsage all read these lists, and a metric left out
// of one of them is the failure mode this consolidation removes.
func TestCgroupMetricsCoversTheUsageMetrics(t *testing.T) {
	assert.Len(t, cgroupMetrics, len(cgroupUsageMetrics)+1, "cgroupMetrics is the usage metrics plus exists")
	for _, metric := range cgroupUsageMetrics {
		assert.NotNil(t, metric)
		assert.NotSame(t, pgCgroupExists, metric, "exists is not a usage metric, forgetUsage must not drop it")
	}
}

// TestUnlimitedRendersAsPlusInf is about the choice parseLimit makes, not
// about the library: it records that +Inf survives the text exposition
// format, so "unlimited" is a value a query can ask about rather than a
// missing series indistinguishable from a collection failure.
func TestUnlimitedRendersAsPlusInf(t *testing.T) {
	reg := prometheus.NewRegistry()
	vec := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "m"}, []string{"p"})
	reg.MustRegister(vec)
	v, err := parseLimit("max")
	if err != nil {
		t.Fatal(err)
	}
	vec.WithLabelValues("a").Set(v)
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	enc := expfmt.NewEncoder(&sb, expfmt.NewFormat(expfmt.TypeTextPlain))
	for _, f := range families {
		if err := enc.Encode(f); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("exposition:\n%s", sb.String())
	if !strings.Contains(sb.String(), "+Inf") {
		t.Fatalf("expected +Inf")
	}
}
