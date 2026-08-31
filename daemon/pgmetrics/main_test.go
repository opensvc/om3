package pgmetrics

import (
	"math"
	"path/filepath"
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

func TestParseCPUMax(t *testing.T) {
	// The default on every cgroup v2 node: not throttled.
	quota, period, err := parseCPUMax("max 100000\n")
	require.NoError(t, err)
	assert.True(t, math.IsInf(quota, 1), "an unthrottled quota must be +Inf, got %v", quota)
	assert.Equal(t, float64(100000), period)

	quota, period, err = parseCPUMax("200000 100000\n")
	require.NoError(t, err)
	assert.Equal(t, float64(200000), quota)
	assert.Equal(t, float64(100000), period)

	_, _, err = parseCPUMax("100000\n")
	assert.Error(t, err, "one field is not a cpu.max")
}

func TestCFSQuota(t *testing.T) {
	// cgroup v1 says -1 for no throttling where v2 says max. Both come
	// out +Inf so a query does not have to know which node it is on.
	assert.True(t, math.IsInf(cfsQuota(-1), 1))
	assert.Equal(t, float64(200000), cfsQuota(200000))
}

func TestParseIOWeight(t *testing.T) {
	// cgroup v2, which is what made cgroup_blkio_weight empty: the bare
	// number parse rejected the "default " prefix.
	v, err := parseIOWeight("default 100\n")
	require.NoError(t, err)
	assert.Equal(t, uint64(100), v)

	v, err = parseIOWeight("default 250\n8:0 200\n8:16 300\n")
	require.NoError(t, err)
	assert.Equal(t, uint64(250), v, "per device lines follow the default and are not it")

	// cgroup v1 blkio.weight is a bare number.
	v, err = parseIOWeight("500\n")
	require.NoError(t, err)
	assert.Equal(t, uint64(500), v)

	_, err = parseIOWeight("default\n")
	assert.Error(t, err)
}

// TestParsersAcceptThisHostsCgroupFiles is the check that would have
// caught every one of these: it reads the real files rather than the ones
// the author imagined, and fails if a parser rejects what the kernel
// actually writes. It skips where there is no opensvc cgroup tree.
func TestParsersAcceptThisHostsCgroupFiles(t *testing.T) {
	dirs, err := filepath.Glob(filepath.Join(CgroupRoot, "*.slice"))
	if err != nil || len(dirs) == 0 {
		t.Skipf("no cgroup under %s to read", CgroupRoot)
	}
	dir := dirs[0]
	t.Logf("reading %s", dir)
	checked := 0
	for name, parse := range map[string]func(string) error{
		"memory.max":     func(s string) error { _, err := parseLimit(s); return err },
		"memory.current": func(s string) error { _, err := parseUint(s); return err },
		"cpu.max":        func(s string) error { _, _, err := parseCPUMax(s); return err },
		"cpu.weight":     func(s string) error { _, err := parseUint(s); return err },
		"io.weight":      func(s string) error { _, err := parseIOWeight(s); return err },
	} {
		content, err := readFile(dir, name)
		if err != nil {
			t.Logf("%s: absent on this host", name)
			continue
		}
		checked++
		assert.NoError(t, parse(content), "%s holds %q, which the parser rejects", name, strings.TrimSpace(content))
	}
	assert.Greater(t, checked, 0, "no cgroup file was readable, the test proved nothing")
}
