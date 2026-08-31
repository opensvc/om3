// Package pgmetrics provides Prometheus metrics for cgroup resource usage
// under the opensvc.slice hierarchy.
//
// It exposes cgroup metrics (CPU, memory, etc.) for all cgroups created by
// the pg_* object keywords.
package pgmetrics

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/metricsreg"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
	"github.com/opensvc/om3/v3/util/systemd"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	// CgroupRoot is the root path for opensvc cgroups
	CgroupRoot = "/sys/fs/cgroup/opensvc.slice"
)

// forgeCgroupPath creates the expected cgroup filesystem path from an object path
// This mimics the structure created by the pg system
func forgeCgroupPath(objPath naming.Path) string {
	var parts []string
	parts = append(parts, CgroupRoot)

	// Handle namespace
	if objPath.Namespace != "" && objPath.Namespace != naming.NsRoot {
		nsName := systemd.Escape("ns." + objPath.Namespace)
		parts = append(parts, "opensvc-"+nsName+".slice")
	}

	// Handle object kind and name
	objName := systemd.Escape(fmt.Sprintf("%s.%s", objPath.Kind, objPath.Name))
	if objPath.Namespace != "" && objPath.Namespace != naming.NsRoot {
		// For namespaced objects: opensvc-ns.<ns>-<kind>.<name>
		nsPrefix := systemd.Escape("ns." + objPath.Namespace)
		parts = append(parts, "opensvc-"+nsPrefix+"-"+objName+".slice")
	} else {
		// For root namespace objects: opensvc-<kind>.<name>
		parts = append(parts, "opensvc-"+objName+".slice")
	}

	return filepath.Join(parts...)
}

// Metrics for cgroup resource usage
var (
	// pgCgroupCPUUsage reports CPU usage in usec for each cgroup
	pgCgroupCPUUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "opensvc",
			Subsystem: "pg",
			Name:      "cgroup_cpu_usage_usec",
			Help:      "Total CPU usage in microseconds for the cgroup",
		},
		[]string{"namespace", "path"},
	)

	// pgCgroupCPUUserUsage reports user CPU usage in usec for each cgroup
	pgCgroupCPUUserUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "opensvc",
			Subsystem: "pg",
			Name:      "cgroup_cpu_user_usage_usec",
			Help:      "Total user CPU usage in microseconds for the cgroup",
		},
		[]string{"namespace", "path"},
	)

	// pgCgroupCPUSystemUsage reports system CPU usage in usec for each cgroup
	pgCgroupCPUSystemUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "opensvc",
			Subsystem: "pg",
			Name:      "cgroup_cpu_system_usage_usec",
			Help:      "Total system CPU usage in microseconds for the cgroup",
		},
		[]string{"namespace", "path"},
	)

	// pgCgroupMemoryCurrent reports current memory usage in bytes for each cgroup
	pgCgroupMemoryCurrent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "opensvc",
			Subsystem: "pg",
			Name:      "cgroup_memory_current_bytes",
			Help:      "Current memory usage in bytes for the cgroup",
		},
		[]string{"namespace", "path"},
	)

	// pgCgroupMemoryMax reports memory max limit in bytes for each cgroup
	pgCgroupMemoryMax = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "opensvc",
			Subsystem: "pg",
			Name:      "cgroup_memory_max_bytes",
			Help:      "Memory max limit in bytes for the cgroup (+Inf = unlimited)",
		},
		[]string{"namespace", "path"},
	)

	// pgCgroupMemoryStat reports various memory statistics in bytes for each cgroup
	pgCgroupMemoryStat = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "opensvc",
			Subsystem: "pg",
			Name:      "cgroup_memory_stat_bytes",
			Help:      "Memory statistics in bytes for the cgroup",
		},
		[]string{"namespace", "path", "stat"},
	)

	// pgCgroupMemoryStatPages reports page-based memory statistics for each cgroup
	pgCgroupMemoryStatPages = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "opensvc",
			Subsystem: "pg",
			Name:      "cgroup_memory_stat_pages",
			Help:      "Page-based memory statistics for the cgroup",
		},
		[]string{"namespace", "path", "stat"},
	)

	// pgCgroupCPUStat reports CPU statistics for each cgroup
	pgCgroupCPUStat = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "opensvc",
			Subsystem: "pg",
			Name:      "cgroup_cpu_stat",
			Help:      "CPU statistics for the cgroup",
		},
		[]string{"namespace", "path", "stat"},
	)

	// pgCgroupCPUShares reports CPU shares for each cgroup
	pgCgroupCPUShares = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "opensvc",
			Subsystem: "pg",
			Name:      "cgroup_cpu_shares",
			Help:      "CPU shares for the cgroup (cgroup v1, 2-262144, 1024 by default)",
		},
		[]string{"namespace", "path"},
	)

	// pgCgroupCPUWeight reports the cgroup v2 cpu weight. It is not
	// cgroup_cpu_shares under another name: the scales differ, so folding
	// the two into one metric would make its value mean different things
	// depending on the host it came from.
	pgCgroupCPUWeight = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "opensvc",
			Subsystem: "pg",
			Name:      "cgroup_cpu_weight",
			Help:      "CPU weight for the cgroup (cgroup v2, 1-10000, 100 by default)",
		},
		[]string{"namespace", "path"},
	)

	// pgCgroupCPUQuota reports CPU quota for each cgroup
	pgCgroupCPUQuota = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "opensvc",
			Subsystem: "pg",
			Name:      "cgroup_cpu_quota",
			Help:      "CPU quota in microseconds per period for the cgroup (+Inf = not throttled)",
		},
		[]string{"namespace", "path"},
	)

	// pgCgroupCPUPeriod reports CPU period for each cgroup
	pgCgroupCPUPeriod = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "opensvc",
			Subsystem: "pg",
			Name:      "cgroup_cpu_period",
			Help:      "CPU period in microseconds for the cgroup",
		},
		[]string{"namespace", "path"},
	)

	// pgCgroupCPUCpus reports CPU cpus allowed for each cgroup
	pgCgroupCPUCpus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "opensvc",
			Subsystem: "pg",
			Name:      "cgroup_cpu_cpus",
			Help:      "CPUs allowed for the cgroup (count)",
		},
		[]string{"namespace", "path"},
	)

	// pgCgroupBlkioWeight reports block IO weight for each cgroup
	pgCgroupBlkioWeight = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "opensvc",
			Subsystem: "pg",
			Name:      "cgroup_blkio_weight",
			Help:      "Block IO default weight for the cgroup",
		},
		[]string{"namespace", "path"},
	)

	// pgCgroupExists reports whether a cgroup exists (1) or not (0)
	pgCgroupExists = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "opensvc",
			Subsystem: "pg",
			Name:      "cgroup_exists",
			Help:      "Whether the cgroup exists (1) or not (0)",
		},
		[]string{"namespace", "path"},
	)
)

// The hint metrics, on the default registry. Every metric above carries a
// path label and so grows with the cluster, which is why they are served
// apart, at /metrics/pg. These three are what tells you to go look.
var (
	// pgCgroups is the size of the detail set: how many series /metrics/pg
	// is carrying, in units of cgroups.
	pgCgroups = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "opensvc",
			Subsystem: "pg",
			Name:      "cgroups",
			Help:      "The number of cgroups reported at /metrics/pg",
		})

	// pgObjectsWithoutCgroup counts the objects that have none. A rise
	// means objects stopped, or pg configuration did not take effect.
	pgObjectsWithoutCgroup = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "opensvc",
			Subsystem: "pg",
			Name:      "objects_without_cgroup",
			Help:      "The number of objects with no cgroup",
		})

	// pgMemoryUtilizationMax is the highest memory.current/memory.max of
	// the cgroups that have a limit, so one series says whether anything
	// is near being reclaimed against. Cgroups with no limit do not
	// count, their ratio being zero over an infinity.
	pgMemoryUtilizationMax = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "opensvc",
			Subsystem: "pg",
			Name:      "cgroup_memory_utilization_max_ratio",
			Help:      "The highest memory usage over memory limit ratio among the limited cgroups",
		})
)

var (
	// cgroupUsageMetrics are the series describing a cgroup that is there.
	// They are what has to be dropped when one goes away, and listing them
	// once is what keeps register, unregister and drop from drifting apart.
	cgroupUsageMetrics = []*prometheus.GaugeVec{
		pgCgroupCPUUsage,
		pgCgroupCPUUserUsage,
		pgCgroupCPUSystemUsage,
		pgCgroupMemoryCurrent,
		pgCgroupMemoryMax,
		pgCgroupMemoryStat,
		pgCgroupMemoryStatPages,
		pgCgroupCPUStat,
		pgCgroupCPUShares,
		pgCgroupCPUWeight,
		pgCgroupCPUQuota,
		pgCgroupCPUPeriod,
		pgCgroupCPUCpus,
		pgCgroupBlkioWeight,
	}

	// cgroupMetrics is every metric this package registers.
	cgroupMetrics = append([]*prometheus.GaugeVec{pgCgroupExists}, cgroupUsageMetrics...)
)

// cgroupKey identifies the object a cgroup belongs to, and is the label
// pair every metric here carries.
type cgroupKey struct {
	namespace string
	path      string
}

func (k cgroupKey) labels() prometheus.Labels {
	return prometheus.Labels{"namespace": k.namespace, "path": k.path}
}

// forgetUsage drops every series describing k's cgroup. The exists series
// is left alone: it is the one that reports the cgroup is gone.
func forgetUsage(k cgroupKey) {
	labels := k.labels()
	for _, metric := range cgroupUsageMetrics {
		metric.DeletePartialMatch(labels)
	}
}

// Manager manages the collection and reporting of cgroup metrics
type Manager struct {
	ctx       context.Context
	cancel    context.CancelFunc
	log       *plog.Logger
	localhost string
	sub       *pubsub.Subscription
	subQS     pubsub.QueueSizer

	// exported is the objects the last collection published a series for,
	// so the next one can tell which have since disappeared.
	exported map[cgroupKey]struct{}

	wg sync.WaitGroup
}

// New creates a new pgmetrics manager with the specified queue size
func New(subQS pubsub.QueueSizer) *Manager {
	localhost := hostname.Hostname()
	return &Manager{
		localhost: localhost,
		subQS:     subQS,
		exported:  make(map[cgroupKey]struct{}),
		log: plog.NewDefaultLogger().
			Attr("pkg", "daemon/pgmetrics").
			WithPrefix("daemon: pgmetrics: "),
	}
}

// Start starts the manager goroutine
func (m *Manager) Start(parent context.Context) error {
	m.log.Infof("starting")
	defer m.log.Infof("started")

	m.ctx, m.cancel = context.WithCancel(parent)

	// Register prometheus metrics
	m.registerMetrics()

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		defer m.log.Infof("stopped")
		m.collectLoop()
	}()

	return nil
}

// Stop stops the manager
func (m *Manager) Stop() error {
	m.log.Infof("stopping")
	defer m.log.Infof("stopped")
	m.cancel()
	m.wg.Wait()
	m.unregisterMetrics()
	return nil
}

func (m *Manager) unregisterMetrics() {
	for _, metric := range cgroupMetrics {
		metricsreg.PG.Unregister(metric)
	}
}

func (m *Manager) registerMetrics() {
	for _, metric := range cgroupMetrics {
		metricsreg.PG.MustRegister(metric)
	}
}

func (m *Manager) collectLoop() {
	// Initial collection
	m.collect()

	// Then collect periodically - every 15 seconds
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.collect()
		}
	}
}

func (m *Manager) collect() {
	if _, err := os.Stat(CgroupRoot); os.IsNotExist(err) {
		m.log.Tracef("cgroup root %s does not exist, skipping collection", CgroupRoot)
		return
	}

	// Get all object paths from the object status data
	objectPaths := object.StatusData.GetPaths()

	// Iterate over object paths and forge their expected cgroup paths
	seen := make(map[cgroupKey]struct{}, len(objectPaths))
	cgroups, withoutCgroup, utilizationMax := 0, 0, 0.0
	for _, objPath := range objectPaths {
		key := cgroupKey{namespace: objPath.Namespace, path: objPath.String()}
		seen[key] = struct{}{}

		// Forge the expected cgroup path from the object path
		cgroupPath := forgeCgroupPath(objPath)

		// Check if the cgroup path exists
		if _, err := os.Stat(cgroupPath); os.IsNotExist(err) {
			// The object has no cgroup at the moment. Report that, and
			// drop the series a previous collection left behind, which
			// would otherwise go on serving their last values and read
			// as a cgroup that is still running.
			pgCgroupExists.WithLabelValues(key.namespace, key.path).Set(0)
			forgetUsage(key)
			withoutCgroup++
			continue
		}

		// Set the exists metric
		pgCgroupExists.WithLabelValues(key.namespace, key.path).Set(1)

		// Collect all metrics for this cgroup
		cgroups++
		if utilization := m.collectCgroupMetrics(cgroupPath, key.namespace, key.path); utilization > utilizationMax {
			utilizationMax = utilization
		}
	}
	pgCgroups.Set(float64(cgroups))
	pgObjectsWithoutCgroup.Set(float64(withoutCgroup))
	pgMemoryUtilizationMax.Set(utilizationMax)

	// An object gone from the cluster keeps nothing, not even an exists
	// series. Prometheus vectors never forget a label combination on
	// their own, so without this the endpoint grows for as long as the
	// daemon runs, and every object it ever saw stays in it.
	for key := range m.exported {
		if _, ok := seen[key]; ok {
			continue
		}
		forgetUsage(key)
		pgCgroupExists.DeletePartialMatch(key.labels())
	}
	m.exported = seen
}

// collectCgroupMetrics publishes one cgroup's series and returns its
// memory usage over its memory limit, or 0 when it has no limit or either
// value could not be read. The caller keeps the highest, which is the one
// series on the default registry that says whether to come and look here.
func (m *Manager) collectCgroupMetrics(cgroupPath, namespace, objPath string) (memoryUtilization float64) {
	// Read cpu.stat
	if cpuStat, err := readFile(cgroupPath, "cpu.stat"); err == nil {
		parseCPUStat(cpuStat, namespace, objPath)
	}

	// Read memory.current
	var memCurrentVal, memMaxVal float64
	var haveCurrent, haveMax bool
	if memCurrent, err := readFile(cgroupPath, "memory.current"); err == nil {
		if val, err := parseUint(memCurrent); err == nil {
			pgCgroupMemoryCurrent.WithLabelValues(namespace, objPath).Set(float64(val))
			memCurrentVal, haveCurrent = float64(val), true
		}
	}

	// Read memory.max
	if memMax, err := readFile(cgroupPath, "memory.max"); err == nil {
		if val, err := parseLimit(memMax); err == nil {
			pgCgroupMemoryMax.WithLabelValues(namespace, objPath).Set(val)
			memMaxVal, haveMax = val, true
		}
	}
	if haveCurrent && haveMax && !math.IsInf(memMaxVal, 1) && memMaxVal > 0 {
		memoryUtilization = memCurrentVal / memMaxVal
	}

	// Read memory.stat
	if memStat, err := readFile(cgroupPath, "memory.stat"); err == nil {
		parseMemoryStat(memStat, namespace, objPath)
	}

	// Read cpu.weight (cgroup v2) or cpu.shares (cgroup v1), into their
	// own metrics, the two not being the same scale.
	if cpuWeight, err := readFile(cgroupPath, "cpu.weight"); err == nil {
		if val, err := parseUint(cpuWeight); err == nil {
			pgCgroupCPUWeight.WithLabelValues(namespace, objPath).Set(float64(val))
		}
	}
	if cpuShares, err := readFile(cgroupPath, "cpu.shares"); err == nil {
		if val, err := parseUint(cpuShares); err == nil {
			pgCgroupCPUShares.WithLabelValues(namespace, objPath).Set(float64(val))
		}
	}

	// Read cpu.max (cgroup v2), which carries the quota and the period in
	// one file, or the pair of cgroup v1 files that hold them separately.
	// Both are microseconds, so they feed the same two metrics.
	if cpuMax, err := readFile(cgroupPath, "cpu.max"); err == nil {
		if quota, period, err := parseCPUMax(cpuMax); err == nil {
			pgCgroupCPUQuota.WithLabelValues(namespace, objPath).Set(quota)
			pgCgroupCPUPeriod.WithLabelValues(namespace, objPath).Set(period)
		}
	} else {
		if cpuQuota, err := readFile(cgroupPath, "cpu.cfs_quota_us"); err == nil {
			if val, err := parseInt(cpuQuota); err == nil {
				pgCgroupCPUQuota.WithLabelValues(namespace, objPath).Set(cfsQuota(val))
			}
		}
		if cpuPeriod, err := readFile(cgroupPath, "cpu.cfs_period_us"); err == nil {
			if val, err := parseUint(cpuPeriod); err == nil {
				pgCgroupCPUPeriod.WithLabelValues(namespace, objPath).Set(float64(val))
			}
		}
	}

	// Read cpuset.cpus
	if cpusetCPUs, err := readFile(cgroupPath, "cpuset.cpus"); err == nil {
		cpus := strings.TrimSpace(cpusetCPUs)
		count := countCPUs(cpus)
		pgCgroupCPUCpus.WithLabelValues(namespace, objPath).Set(float64(count))
	}

	// Read io.weight (for cgroup v2) or blkio.weight (for cgroup v1)
	if ioWeight, err := readFile(cgroupPath, "io.weight"); err == nil {
		if val, err := parseIOWeight(ioWeight); err == nil {
			pgCgroupBlkioWeight.WithLabelValues(namespace, objPath).Set(float64(val))
		}
	} else if blkioWeight, err := readFile(cgroupPath, "blkio.weight"); err == nil {
		if val, err := parseUint(blkioWeight); err == nil {
			pgCgroupBlkioWeight.WithLabelValues(namespace, objPath).Set(float64(val))
		}
	}

	return memoryUtilization
}

func parseCPUStat(content, namespace, objPath string) {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		statName := parts[0]
		statValue, err := parseUint(parts[1])
		if err != nil {
			continue
		}

		// Set the specific metric based on stat name
		switch statName {
		case "usage_usec":
			pgCgroupCPUUsage.WithLabelValues(namespace, objPath).Set(float64(statValue))
		case "user_usec":
			pgCgroupCPUUserUsage.WithLabelValues(namespace, objPath).Set(float64(statValue))
		case "system_usec":
			pgCgroupCPUSystemUsage.WithLabelValues(namespace, objPath).Set(float64(statValue))
		default:
			// Other CPU stats
			pgCgroupCPUStat.WithLabelValues(namespace, objPath, statName).Set(float64(statValue))
		}
	}
}

func parseMemoryStat(content, namespace, objPath string) {
	// Page-based memory statistics (in pages, not bytes)
	// These are page counters that should have the _pages suffix
	pageStats := map[string]bool{
		"workingset_refault_anon":  true,
		"workingset_refault_file":  true,
		"workingset_activate_anon": true,
		"workingset_activate_file": true,
		"workingset_restore_anon":  true,
		"workingset_restore_file":  true,
		"workingset_nodereclaim":   true,
		"pgscan":                   true,
		"pgsteal":                  true,
		"pgscan_kswapd":            true,
		"pgscan_direct":            true,
		"pgscan_khugepaged":        true,
		"pgsteal_kswapd":           true,
		"pgsteal_direct":           true,
		"pgsteal_khugepaged":       true,
		"pgfault":                  true,
		"pgmajfault":               true,
		"pgrefill":                 true,
		"pgactivate":               true,
		"pgdeactivate":             true,
		"pglazyfree":               true,
		"pglazyfreed":              true,
		"zswpin":                   true,
		"zswpout":                  true,
		"zswpwb":                   true,
		"thp_fault_alloc":          true,
		"thp_collapse_alloc":       true,
		"thp_swpout":               true,
		"thp_swpout_fallback":      true,
	}

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		statName := parts[0]
		statValue, err := parseUint(parts[1])
		if err != nil {
			continue
		}

		// Route to appropriate metric based on whether it's page-based or byte-based
		if pageStats[statName] {
			pgCgroupMemoryStatPages.WithLabelValues(namespace, objPath, statName).Set(float64(statValue))
		} else {
			pgCgroupMemoryStat.WithLabelValues(namespace, objPath, statName).Set(float64(statValue))
		}
	}
}

func readFile(dir, filename string) (string, error) {
	path := filepath.Join(dir, filename)
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func parseUint(s string) (uint64, error) {
	return strconv.ParseUint(strings.TrimSpace(s), 10, 64)
}

// parseLimit parses a cgroup v2 limit file, which holds a number or the
// literal "max" when the resource is not limited.
//
// Unlimited comes back as +Inf, which is what prometheus uses for no
// limit. Returning an error instead, as parsing it as a number did, left
// the series unpublished on every cgroup that had no limit set, which is
// all of them by default: cgroup_memory_max_bytes was empty. And 0 would
// not do either, being indistinguishable from a limit of zero.
func parseLimit(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "max" {
		return math.Inf(1), nil
	}
	v, err := parseUint(s)
	if err != nil {
		return 0, err
	}
	return float64(v), nil
}

// parseCPUMax parses cgroup v2 cpu.max, which is "$QUOTA $PERIOD" in
// microseconds, $QUOTA being the literal "max" on a cgroup that is not
// throttled.
func parseCPUMax(s string) (quota, period float64, err error) {
	fields := strings.Fields(s)
	if len(fields) != 2 {
		return 0, 0, fmt.Errorf("cpu.max: want 2 fields, got %d in %q", len(fields), strings.TrimSpace(s))
	}
	if quota, err = parseLimit(fields[0]); err != nil {
		return 0, 0, err
	}
	if period, err = parseLimit(fields[1]); err != nil {
		return 0, 0, err
	}
	return quota, period, nil
}

// cfsQuota converts a cgroup v1 cpu.cfs_quota_us value, which uses -1 for
// no throttling, to the +Inf cgroup v2 and every other limit here report,
// so that one query works whichever version the node runs.
func cfsQuota(v int64) float64 {
	if v < 0 {
		return math.Inf(1)
	}
	return float64(v)
}

// parseIOWeight reads the default weight out of io.weight, whose cgroup
// v2 form is "default $WEIGHT" followed by optional per device lines,
// where the cgroup v1 blkio.weight is a bare number.
func parseIOWeight(s string) (uint64, error) {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	fields := strings.Fields(line)
	switch {
	case len(fields) == 2 && fields[0] == "default":
		return parseUint(fields[1])
	case len(fields) == 1:
		return parseUint(fields[0])
	}
	return 0, fmt.Errorf("io.weight: no default weight in %q", line)
}

func parseInt(s string) (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(s), 10, 64)
}

// countCPUs counts the number of CPUs in a cpuset.cpus string
// Format: "0-3,5,7-9" -> 7 CPUs (0,1,2,3,5,7,8,9)
func countCPUs(cpus string) int {
	if cpus == "" {
		return 0
	}
	count := 0
	ranges := strings.Split(cpus, ",")
	for _, r := range ranges {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		if strings.Contains(r, "-") {
			parts := strings.Split(r, "-")
			if len(parts) == 2 {
				start, err1 := strconv.Atoi(parts[0])
				end, err2 := strconv.Atoi(parts[1])
				if err1 == nil && err2 == nil {
					count += end - start + 1
				}
			}
		} else {
			if _, err := strconv.Atoi(r); err == nil {
				count++
			}
		}
	}
	return count
}
