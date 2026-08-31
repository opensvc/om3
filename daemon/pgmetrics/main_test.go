package pgmetrics

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/util/metricsreg"
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

// TestMetricsGoToTheDetailRegistry pins which registry the per cgroup
// series land in. They carry a path label, so on a large cluster they are
// most of what /metrics used to cost, and they are served at /metrics/pg
// instead.
func TestMetricsGoToTheDetailRegistry(t *testing.T) {
	m := New(nil)
	m.registerMetrics()
	t.Cleanup(m.unregisterMetrics)

	for _, metric := range cgroupMetrics {
		name := metricName(t, metric)
		assert.True(t, isRegistered(metricsreg.PG, name), "%s must be served at /metrics/pg", name)
		assert.False(t, isRegistered(prometheus.DefaultRegisterer, name), "%s must not be on /metrics", name)
	}

	// The hints stay where the normal scrape finds them.
	for _, name := range []string{
		"opensvc_pg_cgroups",
		"opensvc_pg_objects_without_cgroup",
		"opensvc_pg_cgroup_memory_utilization_max_ratio",
	} {
		assert.True(t, isRegistered(prometheus.DefaultRegisterer, name), "%s is the hint, it belongs on /metrics", name)
		assert.False(t, isRegistered(metricsreg.PG, name), "%s must not also be at /metrics/pg", name)
	}
}

func isRegistered(reg prometheus.Registerer, name string) bool {
	probe := prometheus.NewGauge(prometheus.GaugeOpts{Name: name, Help: "probe"})
	if err := reg.Register(probe); err != nil {
		return true
	}
	reg.Unregister(probe)
	return false
}

func metricName(t *testing.T, vec *prometheus.GaugeVec) string {
	t.Helper()
	ch := make(chan *prometheus.Desc, 1)
	vec.Describe(ch)
	close(ch)
	desc := <-ch
	// Desc.String() is "Desc{fqName: "x", ...}", the fqName being what a
	// registry keys on.
	s := desc.String()
	start := strings.Index(s, `fqName: "`) + len(`fqName: "`)
	return s[start : start+strings.Index(s[start:], `"`)]
}

// withFakeCgroupTree points CgroupRoot at a temp dir and populates
// object.StatusData, so collect() can be driven without a live daemon.
func withFakeCgroupTree(t testing.TB, paths []string, withCgroup []string) *Manager {
	t.Helper()
	root := t.TempDir()
	wasRoot, wasData := CgroupRoot, object.StatusData
	CgroupRoot = root
	object.StatusData = object.NewData[object.Status]()
	t.Cleanup(func() { CgroupRoot, object.StatusData = wasRoot, wasData })

	for _, ps := range paths {
		p, err := naming.ParsePath(ps)
		require.NoError(t, err)
		object.StatusData.Set(p, &object.Status{})
	}
	for _, ps := range withCgroup {
		p, err := naming.ParsePath(ps)
		require.NoError(t, err)
		dir := forgeCgroupPath(p)
		require.NoError(t, os.MkdirAll(dir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "memory.current"), []byte("16384\n"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "memory.max"), []byte("max\n"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "cpu.max"), []byte("max 100000\n"), 0644))
	}

	m := New(nil)
	m.registerMetrics()
	t.Cleanup(m.unregisterMetrics)
	return m
}

func TestCollectDropsTheSeriesOfAVanishedCgroup(t *testing.T) {
	m := withFakeCgroupTree(t, []string{"ns1/svc/a", "ns1/svc/b"}, []string{"ns1/svc/a"})

	m.collect()
	assert.Equal(t, 1, countSeries(pgCgroupMemoryCurrent), "only the object with a cgroup reports usage")
	assert.Equal(t, 2, countSeries(pgCgroupExists), "both objects report whether they have one")

	p, err := naming.ParsePath("ns1/svc/a")
	require.NoError(t, err)
	require.NoError(t, os.RemoveAll(forgeCgroupPath(p)))

	m.collect()
	assert.Equal(t, 0, countSeries(pgCgroupMemoryCurrent), "the usage series must go with the cgroup")
	assert.Equal(t, 2, countSeries(pgCgroupExists), "and exists must stay, to report it is gone")

	// The object leaves the cluster: nothing of it is left.
	object.StatusData.Unset(p)
	m.collect()
	assert.Equal(t, 1, countSeries(pgCgroupExists))
}

// BenchmarkCollectWithoutCgroups pins what a regression already cost once.
// forgetUsage was called on every tick for every object with no cgroup,
// and DeletePartialMatch walks every series of every vector, since a
// partial label match cannot be indexed. On a cluster where 79 of 103
// objects have no cgroup that was 9.4% of the daemon's cpu, measured. It
// is only called on the transition now, so this should not scale with the
// number of objects that have none.
func BenchmarkCollectWithoutCgroups(b *testing.B) {
	paths := make([]string, 0, 100)
	for i := range 100 {
		paths = append(paths, fmt.Sprintf("ns1/svc/o%d", i))
	}
	m := withFakeCgroupTree(b, paths, paths[:20])
	m.collect()
	b.ResetTimer()
	for b.Loop() {
		m.collect()
	}
}
