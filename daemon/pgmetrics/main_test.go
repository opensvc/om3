package pgmetrics

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/util/metricsreg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countSeries counts the metrics a collector emits. Done by hand rather
// than with prometheus' testutil, which is not in the module graph and
// would cost a go mod tidy to add.
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
		for name, content := range map[string]string{
			"memory.current": "16384\n",
			"memory.max":     "65536\n",
			"cpu.max":        "max 100000\n",
			"cpu.weight":     "100\n",
			"io.weight":      "default 100\n",
			"cpu.stat":       "usage_usec 1000\nuser_usec 600\nsystem_usec 400\nnr_periods 3\n",
			"memory.stat":    "anon 4096\nfile 8192\npgfault 12\n",
		} {
			require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0644))
		}
	}
	return New()
}

func isRegistered(reg prometheus.Registerer, name string) bool {
	probe := prometheus.NewGauge(prometheus.GaugeOpts{Name: name, Help: "probe"})
	if err := reg.Register(probe); err != nil {
		return true
	}
	reg.Unregister(probe)
	return false
}

func TestCollectorsGoToTheRightRegistries(t *testing.T) {
	m := New()
	m.registerMetrics()
	t.Cleanup(m.unregisterMetrics)

	for _, desc := range detailDescs {
		name := descName(t, desc)
		assert.True(t, isRegistered(metricsreg.PG, name), "%s must be served at /metrics/pg", name)
		assert.False(t, isRegistered(prometheus.DefaultRegisterer, name), "%s must not be on /metrics", name)
	}
	for _, name := range []string{
		"opensvc_pg_cgroups",
		"opensvc_pg_objects_without_cgroup",
		"opensvc_pg_cgroup_memory_utilization_max_ratio",
	} {
		assert.True(t, isRegistered(prometheus.DefaultRegisterer, name), "%s is the hint, it belongs on /metrics", name)
		assert.False(t, isRegistered(metricsreg.PG, name), "%s must not also be at /metrics/pg", name)
	}
}

func descName(t *testing.T, desc *prometheus.Desc) string {
	t.Helper()
	s := desc.String()
	start := strings.Index(s, `fqName: "`) + len(`fqName: "`)
	return s[start : start+strings.Index(s[start:], `"`)]
}

// TestAVanishedCgroupLeavesNoSeries is what used to need a deletion pass
// over every vector, and a pair of maps to know which cgroups to run it
// for. A walk emits metrics for what it found, so a cgroup that is gone
// is simply not in the next one.
func TestAVanishedCgroupLeavesNoSeries(t *testing.T) {
	m := withFakeCgroupTree(t, []string{"ns1/svc/a", "ns1/svc/b"}, []string{"ns1/svc/a"})

	s := m.snapshot(true)
	assert.Equal(t, 1, s.cgroups)
	assert.Equal(t, 1, s.withoutCgroup)
	assert.Contains(t, metricNames(s.detail), "opensvc_pg_cgroup_memory_current_bytes")

	p, err := naming.ParsePath("ns1/svc/a")
	require.NoError(t, err)
	require.NoError(t, os.RemoveAll(forgeCgroupPath(p)))

	m.snap = nil // the cache would otherwise still hold the walk above
	s = m.snapshot(true)
	assert.Equal(t, 0, s.cgroups)
	assert.Equal(t, 2, s.withoutCgroup)
	assert.NotContains(t, metricNames(s.detail), "opensvc_pg_cgroup_memory_current_bytes",
		"the usage series must go with the cgroup")
	assert.Contains(t, metricNames(s.detail), "opensvc_pg_cgroup_exists",
		"and exists must stay, to report it is gone")
}

func metricNames(metrics []prometheus.Metric) []string {
	seen := make(map[string]bool, len(metrics))
	l := make([]string, 0, len(metrics))
	for _, metric := range metrics {
		name := metric.Desc().String()
		start := strings.Index(name, `fqName: "`) + len(`fqName: "`)
		name = name[start : start+strings.Index(name[start:], `"`)]
		if !seen[name] {
			seen[name] = true
			l = append(l, name)
		}
	}
	return l
}

// TestTheHintWalkDoesNotPayForTheDetail is the point of the split: a
// scrape of /metrics happens every few seconds and must not open the ten
// files per cgroup that /metrics/pg needs.
func TestTheHintWalkDoesNotPayForTheDetail(t *testing.T) {
	m := withFakeCgroupTree(t, []string{"ns1/svc/a", "ns1/svc/b"}, []string{"ns1/svc/a"})

	s := m.snapshot(false)
	assert.Empty(t, s.detail, "the cheap walk emits no detail")
	assert.Equal(t, 1, s.cgroups, "but still counts the cgroups")
	assert.Equal(t, 1, s.withoutCgroup)
	assert.InDelta(t, 16384.0/65536.0, s.utilizationMax, 1e-9, "and still reads the two files the ratio needs")
}

// TestSnapshotIsReusedWithinMinInterval keeps the walk from happening on
// every scrape: /metrics is scraped far more often than the tree changes.
func TestSnapshotIsReusedWithinMinInterval(t *testing.T) {
	m := withFakeCgroupTree(t, []string{"ns1/svc/a"}, []string{"ns1/svc/a"})

	first := m.snapshot(true)
	assert.Same(t, first, m.snapshot(true), "a second scrape reuses the walk")
	assert.Same(t, first, m.snapshot(false), "and a hint scrape is happy with a full one")

	// A hint walk is not enough for the detail, though.
	cheap := &snapshot{at: time.Now(), full: false}
	m.snap = cheap
	got := m.snapshot(true)
	assert.NotSame(t, cheap, got, "the detail must not be served from a cheap walk")
	assert.True(t, got.full)
	assert.NotEmpty(t, got.detail)
}

func BenchmarkWalkFull(b *testing.B) {
	m := benchTree(b)
	b.ResetTimer()
	for b.Loop() {
		m.walk(true)
	}
}

func BenchmarkWalkHint(b *testing.B) {
	m := benchTree(b)
	b.ResetTimer()
	for b.Loop() {
		m.walk(false)
	}
}

func benchTree(b *testing.B) *Manager {
	paths := make([]string, 0, 100)
	for i := range 100 {
		paths = append(paths, fmt.Sprintf("ns1/svc/o%d", i))
	}
	return withFakeCgroupTree(b, paths, paths[:24])
}
