// Package pgmetrics provides Prometheus metrics for cgroup resource usage
// under the opensvc.slice hierarchy.
//
// It exposes cgroup metrics (CPU, memory, etc.) for the cgroups of the
// process groups: the opensvc slice itself, the slice of each namespace, and
// the slice of each object, told apart by the scope label.
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
	"github.com/opensvc/om3/v3/util/metricsreg"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/systemd"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// CgroupRoot is the root path for opensvc cgroups. A var rather than
	// a const so the tests can point collect() at a tree they built.
	CgroupRoot = "/sys/fs/cgroup/opensvc.slice"

	// BlockRoot holds a directory per block device, named by its
	// major:minor, as io.stat names them. A var for the tests.
	BlockRoot = "/sys/dev/block"

	// HostCgroupRoot is the root of the cgroup hierarchy, which holds the
	// opensvc slice and the other top level slices. ProcRoot and
	// CPUOnlineFile are where the host totals are read. Vars for the
	// tests.
	HostCgroupRoot = "/sys/fs/cgroup"
	ProcRoot       = "/proc"
	CPUOnlineFile  = "/sys/devices/system/cpu/online"
)

// The scope label values: the cgroup a series is of.
const (
	// scopeOpensvc is the opensvc slice, which holds every group of the
	// node and the daemon: its namespace and path labels are empty.
	scopeOpensvc = "opensvc"

	// scopeNamespace is the slice of a namespace, which holds the groups
	// of its objects: its path label is empty. The objects of the root
	// namespace have no namespace slice, they are in the opensvc slice.
	scopeNamespace = "namespace"

	// scopeObject is the slice of an object.
	scopeObject = "object"

	// scopeHost is the whole node, which the opensvc slice is a share
	// of: its namespace and path labels are empty.
	scopeHost = "host"

	// scopeSlice is a top level slice other than the opensvc one, as
	// system.slice, user.slice, machine.slice or init.scope: its path label
	// is its name, its namespace label empty.
	scopeSlice = "slice"
)

// hasCgroup says the objects of kind k have a process group: the kinds with
// resources. The objects of the other kinds, the namespace configurations
// among them, whose group is the namespace slice, have none to report.
func hasCgroup(k naming.Kind) bool {
	return k == naming.KindSvc || k == naming.KindVol
}

// forgeNamespaceCgroupPath returns the cgroup path of the slice of the
// namespace ns, which is not the root one.
func forgeNamespaceCgroupPath(ns string) string {
	return filepath.Join(CgroupRoot, "opensvc-"+systemd.Escape("ns."+ns)+".slice")
}

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
// The metrics are emitted as const metrics by the collectors below,
// built during the walk that reads them, rather than held in vectors a
// timer keeps up to date.
//
// That is what lets the walk happen at scrape time. It also removes a
// class of bug rather than fixing it: a vector remembers every label
// combination it was ever given, so a cgroup that went away had to be
// deleted from each of them by hand, and forgetting to do that left a
// stopped object reporting the values of the last collection that found
// it, for as long as the daemon ran. Const metrics only exist for what
// the current walk found.
var (
	cgroupExistsDesc = prometheus.NewDesc("opensvc_pg_cgroup_exists",
		"Whether the cgroup exists (1) or not (0)", cgroupLabels, nil)
	cpuUsageDesc = prometheus.NewDesc("opensvc_pg_cgroup_cpu_usage_usec",
		"Total CPU usage in microseconds for the cgroup", cgroupLabels, nil)
	cpuUserUsageDesc = prometheus.NewDesc("opensvc_pg_cgroup_cpu_user_usage_usec",
		"Total user CPU usage in microseconds for the cgroup", cgroupLabels, nil)
	cpuSystemUsageDesc = prometheus.NewDesc("opensvc_pg_cgroup_cpu_system_usage_usec",
		"Total system CPU usage in microseconds for the cgroup", cgroupLabels, nil)
	memoryCurrentDesc = prometheus.NewDesc("opensvc_pg_cgroup_memory_current_bytes",
		"Current memory usage in bytes for the cgroup", cgroupLabels, nil)
	memoryMaxDesc = prometheus.NewDesc("opensvc_pg_cgroup_memory_max_bytes",
		"Memory max limit in bytes for the cgroup (+Inf = unlimited)", cgroupLabels, nil)
	cpuSharesDesc = prometheus.NewDesc("opensvc_pg_cgroup_cpu_shares",
		"CPU shares for the cgroup (cgroup v1, 2-262144, 1024 by default)", cgroupLabels, nil)
	cpuWeightDesc = prometheus.NewDesc("opensvc_pg_cgroup_cpu_weight",
		"CPU weight for the cgroup (cgroup v2, 1-10000, 100 by default)", cgroupLabels, nil)
	cpuQuotaDesc = prometheus.NewDesc("opensvc_pg_cgroup_cpu_quota",
		"CPU quota in microseconds per period for the cgroup (+Inf = not throttled)", cgroupLabels, nil)
	cpuPeriodDesc = prometheus.NewDesc("opensvc_pg_cgroup_cpu_period",
		"CPU period in microseconds for the cgroup", cgroupLabels, nil)
	cpuCpusDesc = prometheus.NewDesc("opensvc_pg_cgroup_cpu_cpus",
		"CPUs allowed for the cgroup (count)", cgroupLabels, nil)
	blkioWeightDesc = prometheus.NewDesc("opensvc_pg_cgroup_blkio_weight",
		"Block IO default weight for the cgroup", cgroupLabels, nil)
	memoryStatDesc = prometheus.NewDesc("opensvc_pg_cgroup_memory_stat_bytes",
		"Memory statistics in bytes for the cgroup", cgroupStatLabels, nil)
	memoryStatPagesDesc = prometheus.NewDesc("opensvc_pg_cgroup_memory_stat_pages",
		"Page-based memory statistics for the cgroup", cgroupStatLabels, nil)
	cpuStatDesc = prometheus.NewDesc("opensvc_pg_cgroup_cpu_stat",
		"CPU statistics for the cgroup", cgroupStatLabels, nil)
	ioReadBytesDesc = prometheus.NewDesc("opensvc_pg_cgroup_io_read_bytes",
		"Bytes read by the cgroup, over the block devices no other device is stacked on (cgroup v2)", cgroupLabels, nil)
	ioWriteBytesDesc = prometheus.NewDesc("opensvc_pg_cgroup_io_write_bytes",
		"Bytes written by the cgroup, over the block devices no other device is stacked on (cgroup v2)", cgroupLabels, nil)
	ioReadIOsDesc = prometheus.NewDesc("opensvc_pg_cgroup_io_read_ios",
		"Read operations of the cgroup, over the block devices no other device is stacked on (cgroup v2)", cgroupLabels, nil)
	ioWriteIOsDesc = prometheus.NewDesc("opensvc_pg_cgroup_io_write_ios",
		"Write operations of the cgroup, over the block devices no other device is stacked on (cgroup v2)", cgroupLabels, nil)
	tasksDesc = prometheus.NewDesc("opensvc_pg_cgroup_tasks",
		"The number of tasks in the cgroup", cgroupLabels, nil)

	// detailDescs is what /metrics/pg answers with.
	detailDescs = []*prometheus.Desc{
		cgroupExistsDesc, cpuUsageDesc, cpuUserUsageDesc, cpuSystemUsageDesc,
		memoryCurrentDesc, memoryMaxDesc, cpuSharesDesc, cpuWeightDesc,
		cpuQuotaDesc, cpuPeriodDesc, cpuCpusDesc, blkioWeightDesc,
		memoryStatDesc, memoryStatPagesDesc, cpuStatDesc,
		ioReadBytesDesc, ioWriteBytesDesc, ioReadIOsDesc, ioWriteIOsDesc,
		tasksDesc,
	}
)

var cgroupLabels = []string{"scope", "namespace", "path"}
var cgroupStatLabels = []string{"scope", "namespace", "path", "stat"}

// The hints, on the default registry. Every metric above carries a path
// label and so grows with the cluster, which is why they are served
// apart, at /metrics/pg. These three are what tells you to go look, and
// they are cheap enough to answer on every scrape of /metrics.
var (
	cgroupsDesc = prometheus.NewDesc("opensvc_pg_cgroups",
		"The number of cgroups reported at /metrics/pg", nil, nil)
	objectsWithoutCgroupDesc = prometheus.NewDesc("opensvc_pg_objects_without_cgroup",
		"The number of svc and vol objects with no cgroup (the cgroups that do exist are at /metrics/pg)", nil, nil)
	memoryUtilizationMaxDesc = prometheus.NewDesc("opensvc_pg_cgroup_memory_utilization_max_ratio",
		"The highest memory usage over memory limit ratio among the limited cgroups (per cgroup at /metrics/pg)", nil, nil)
)

// Manager manages the collection and reporting of cgroup metrics
type Manager struct {
	ctx    context.Context
	cancel context.CancelFunc
	log    *plog.Logger

	// mu guards snap, and is held across a walk so that concurrent
	// scrapes wait for the one in flight instead of each starting theirs.
	mu   sync.Mutex
	snap *snapshot

	detail detailCollector
	hint   hintCollector
}

// snapshot is one walk of the cgroup tree.
//
// The detail is built as const metrics during the walk, so a scrape only
// forwards them, and two scrapes can share one snapshot without copying:
// a const metric cannot be modified.
type snapshot struct {
	at     time.Time
	full   bool
	detail []prometheus.Metric

	cgroups        int
	withoutCgroup  int
	utilizationMax float64
}

// minInterval is how old a snapshot may be before a scrape walks the tree
// again. It is the period of the ticker this replaces, so the data is no
// less fresh than it was. What changes is that nothing walks while
// nothing is scraping, and that a detail endpoint scraped every five
// minutes is read every five minutes rather than every fifteen seconds.
var minInterval = 15 * time.Second

// emitter accumulates the const metrics of one walk.
type emitter struct {
	out []prometheus.Metric

	// stacked caches, for the walk, whether a block device has another
	// stacked on it.
	stacked map[string]bool
}

func (e *emitter) gauge(desc *prometheus.Desc, v float64, labels ...string) {
	e.out = append(e.out, prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, v, labels...))
}

// detailCollector answers /metrics/pg, walking the cgroup tree when it is
// scraped rather than on a timer.
type detailCollector struct{ m *Manager }

func (c detailCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range detailDescs {
		ch <- desc
	}
}

func (c detailCollector) Collect(ch chan<- prometheus.Metric) {
	for _, metric := range c.m.snapshot(true).detail {
		ch <- metric
	}
}

// hintCollector answers the pg metrics on /metrics. It asks for the cheap
// walk: the two counts need only a stat per object, and the utilization
// ratio two files per cgroup, where the detail opens ten.
type hintCollector struct{ m *Manager }

func (c hintCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- cgroupsDesc
	ch <- objectsWithoutCgroupDesc
	ch <- memoryUtilizationMaxDesc
}

func (c hintCollector) Collect(ch chan<- prometheus.Metric) {
	s := c.m.snapshot(false)
	ch <- prometheus.MustNewConstMetric(cgroupsDesc, prometheus.GaugeValue, float64(s.cgroups))
	ch <- prometheus.MustNewConstMetric(objectsWithoutCgroupDesc, prometheus.GaugeValue, float64(s.withoutCgroup))
	ch <- prometheus.MustNewConstMetric(memoryUtilizationMaxDesc, prometheus.GaugeValue, s.utilizationMax)
}

// snapshot returns a walk no older than minInterval, doing one when what
// is cached is too old, or was the cheap walk and the detail is wanted.
func (m *Manager) snapshot(full bool) *snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s := m.snap; s != nil && time.Since(s.at) < minInterval && (s.full || !full) {
		return s
	}
	m.snap = m.walk(full)
	return m.snap
}

func (m *Manager) walk(full bool) *snapshot {
	s := &snapshot{at: time.Now(), full: full}
	if _, err := os.Stat(CgroupRoot); os.IsNotExist(err) {
		m.log.Tracef("cgroup root %s does not exist, skipping collection", CgroupRoot)
		return s
	}
	var e emitter
	visit := func(cgroupPath string, labels ...string) bool {
		if _, err := os.Stat(cgroupPath); os.IsNotExist(err) {
			if full {
				e.gauge(cgroupExistsDesc, 0, labels...)
			}
			return false
		}
		s.cgroups++
		if full {
			e.gauge(cgroupExistsDesc, 1, labels...)
		}
		if utilization := m.collectCgroupMetrics(&e, full, cgroupPath, labels...); utilization > s.utilizationMax {
			s.utilizationMax = utilization
		}
		return true
	}
	visit(CgroupRoot, scopeOpensvc, "", "")
	namespaces := make(map[string]struct{})
	for _, objPath := range object.StatusData.GetPaths() {
		if !hasCgroup(objPath.Kind) {
			continue
		}
		if objPath.Namespace != naming.NsRoot {
			namespaces[objPath.Namespace] = struct{}{}
		}
		if !visit(forgeCgroupPath(objPath), scopeObject, objPath.Namespace, objPath.String()) {
			s.withoutCgroup++
		}
	}
	for ns := range namespaces {
		visit(forgeNamespaceCgroupPath(ns), scopeNamespace, ns, "")
	}
	if full {
		for _, name := range otherSlices() {
			visit(filepath.Join(HostCgroupRoot, name), scopeSlice, "", name)
		}
		collectHost(&e)
	}
	s.detail = e.out
	return s
}

// otherSlices returns the names of the top level slices and scopes other
// than the opensvc slice, as systemd-cgtop shows them: the virtual machines
// and containers started by libvirt or podman may be in machine.slice
// rather than in the opensvc one.
func otherSlices() []string {
	entries, err := os.ReadDir(HostCgroupRoot)
	if err != nil {
		return nil
	}
	l := make([]string, 0)
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() || name == filepath.Base(CgroupRoot) {
			continue
		}
		if strings.HasSuffix(name, ".slice") || strings.HasSuffix(name, ".scope") {
			l = append(l, name)
		}
	}
	return l
}

// collectHost emits the totals of the node, the host scope, with the names
// of the cgroup series where they mean the same: the root cgroup has no
// memory.current nor pids.current, so the memory and the task count are the
// ones of the kernel.
//
//   - cpu usage: the cpu.stat of the root cgroup, which counts the whole node
//     on cgroup v2, or /proc/stat.
//   - cpu cpus: the online cpus.
//   - memory current: MemTotal minus MemAvailable, and memory max MemTotal.
//   - io: the io.stat of the root cgroup, the devices stacked on left out.
//   - tasks: the tasks of the node, the total of /proc/loadavg.
func collectHost(e *emitter) {
	labels := []string{scopeHost, "", ""}
	if cpuStat, err := readFile(HostCgroupRoot, "cpu.stat"); err == nil {
		parseCPUStat(e, cpuStat, labels...)
	} else if usec, err := procStatUsageUsec(); err == nil {
		e.gauge(cpuUsageDesc, usec, labels...)
	}
	if online, err := os.ReadFile(CPUOnlineFile); err == nil {
		e.gauge(cpuCpusDesc, float64(countCPUs(strings.TrimSpace(string(online)))), labels...)
	}
	if total, available, err := memInfo(); err == nil {
		e.gauge(memoryCurrentDesc, total-available, labels...)
		e.gauge(memoryMaxDesc, total, labels...)
	}
	if ioStat, err := readFile(HostCgroupRoot, "io.stat"); err == nil {
		parseIOStat(e, ioStat, labels...)
	}
	if loadavg, err := readFile(ProcRoot, "loadavg"); err == nil {
		// "0.10 0.20 0.30 <running>/<total> <last pid>"
		if fields := strings.Fields(loadavg); len(fields) >= 4 {
			if _, total, ok := strings.Cut(fields[3], "/"); ok {
				if val, err := parseUint(total); err == nil {
					e.gauge(tasksDesc, float64(val), labels...)
				}
			}
		}
	}
}

// procStatUsageUsec returns the cpu time the node spent out of idle and
// iowait, in microseconds, from the "cpu" line of /proc/stat, which counts
// in USER_HZ, 100 a second on Linux.
func procStatUsageUsec() (float64, error) {
	b, err := readFile(ProcRoot, "stat")
	if err != nil {
		return 0, err
	}
	line, _, _ := strings.Cut(b, "\n")
	fields := strings.Fields(line)
	if len(fields) < 9 || fields[0] != "cpu" {
		return 0, fmt.Errorf("stat: no cpu line in %q", line)
	}
	var ticks uint64
	// user nice system idle iowait irq softirq steal
	for _, i := range []int{1, 2, 3, 6, 7, 8} {
		v, err := parseUint(fields[i])
		if err != nil {
			return 0, err
		}
		ticks += v
	}
	return float64(ticks) * 1e6 / 100, nil
}

// memInfo returns MemTotal and MemAvailable, in bytes, from /proc/meminfo.
func memInfo() (total, available float64, err error) {
	b, err := readFile(ProcRoot, "meminfo")
	if err != nil {
		return 0, 0, err
	}
	var haveTotal, haveAvailable bool
	for _, line := range strings.Split(b, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		v, err := parseUint(fields[1])
		if err != nil {
			continue
		}
		switch fields[0] {
		case "MemTotal:":
			total, haveTotal = float64(v)*1024, true
		case "MemAvailable:":
			available, haveAvailable = float64(v)*1024, true
		}
	}
	if !haveTotal || !haveAvailable {
		return 0, 0, fmt.Errorf("meminfo: no MemTotal or MemAvailable")
	}
	return total, available, nil
}

// New creates a new pgmetrics manager.
//
// It takes no queue sizer any more: it subscribes to nothing, having no
// goroutine left to receive on. The cgroup tree is read by the scrapes
// that ask for it.
func New() *Manager {
	return &Manager{
		log: plog.NewDefaultLogger().
			Attr("pkg", "daemon/pgmetrics").
			WithPrefix("daemon: pgmetrics: "),
	}
}

// Start registers the collectors. There is no goroutine to start any
// more: the work happens on the scrapes that ask for it.
func (m *Manager) Start(parent context.Context) error {
	m.log.Infof("starting")
	defer m.log.Infof("started")
	m.ctx, m.cancel = context.WithCancel(parent)
	m.registerMetrics()
	return nil
}

func (m *Manager) Stop() error {
	m.log.Infof("stopping")
	defer m.log.Infof("stopped")
	m.cancel()
	m.unregisterMetrics()
	return nil
}

func (m *Manager) registerMetrics() {
	m.detail = detailCollector{m: m}
	m.hint = hintCollector{m: m}
	metricsreg.PG.MustRegister(m.detail)
	prometheus.MustRegister(m.hint)
}

func (m *Manager) unregisterMetrics() {
	metricsreg.PG.Unregister(m.detail)
	prometheus.Unregister(m.hint)
}

// collectCgroupMetrics reads one cgroup and returns its memory usage over
// its memory limit, or 0 when it has no limit or either value could not
// be read.
//
// With full false it reads only the two files that ratio needs and emits
// nothing. That is the walk a scrape of /metrics wants: the hint tells
// you whether to go and look, and it should not cost what looking costs.
func (m *Manager) collectCgroupMetrics(e *emitter, full bool, cgroupPath string, labels ...string) (memoryUtilization float64) {
	// Read memory.current and memory.max first: they are the only files
	// the hint walk opens, and the ratio is computed from them.
	var memCurrentVal, memMaxVal float64
	var haveCurrent, haveMax bool
	if memCurrent, err := readFile(cgroupPath, "memory.current"); err == nil {
		if val, err := parseUint(memCurrent); err == nil {
			if full {
				e.gauge(memoryCurrentDesc, float64(val), labels...)
			}
			memCurrentVal, haveCurrent = float64(val), true
		}
	}

	// Read memory.max
	if memMax, err := readFile(cgroupPath, "memory.max"); err == nil {
		if val, err := parseLimit(memMax); err == nil {
			if full {
				e.gauge(memoryMaxDesc, val, labels...)
			}
			memMaxVal, haveMax = val, true
		}
	}
	if haveCurrent && haveMax && !math.IsInf(memMaxVal, 1) && memMaxVal > 0 {
		memoryUtilization = memCurrentVal / memMaxVal
	}
	if !full {
		return memoryUtilization
	}

	// Read cpu.stat
	if cpuStat, err := readFile(cgroupPath, "cpu.stat"); err == nil {
		parseCPUStat(e, cpuStat, labels...)
	}

	// Read memory.stat
	if memStat, err := readFile(cgroupPath, "memory.stat"); err == nil {
		parseMemoryStat(e, memStat, labels...)
	}

	// Read cpu.weight (cgroup v2) or cpu.shares (cgroup v1), into their
	// own metrics, the two not being the same scale.
	if cpuWeight, err := readFile(cgroupPath, "cpu.weight"); err == nil {
		if val, err := parseUint(cpuWeight); err == nil {
			e.gauge(cpuWeightDesc, float64(val), labels...)
		}
	}
	if cpuShares, err := readFile(cgroupPath, "cpu.shares"); err == nil {
		if val, err := parseUint(cpuShares); err == nil {
			e.gauge(cpuSharesDesc, float64(val), labels...)
		}
	}

	// Read cpu.max (cgroup v2), which carries the quota and the period in
	// one file, or the pair of cgroup v1 files that hold them separately.
	// Both are microseconds, so they feed the same two metrics.
	if cpuMax, err := readFile(cgroupPath, "cpu.max"); err == nil {
		if quota, period, err := parseCPUMax(cpuMax); err == nil {
			e.gauge(cpuQuotaDesc, quota, labels...)
			e.gauge(cpuPeriodDesc, period, labels...)
		}
	} else {
		if cpuQuota, err := readFile(cgroupPath, "cpu.cfs_quota_us"); err == nil {
			if val, err := parseInt(cpuQuota); err == nil {
				e.gauge(cpuQuotaDesc, cfsQuota(val), labels...)
			}
		}
		if cpuPeriod, err := readFile(cgroupPath, "cpu.cfs_period_us"); err == nil {
			if val, err := parseUint(cpuPeriod); err == nil {
				e.gauge(cpuPeriodDesc, float64(val), labels...)
			}
		}
	}

	// Read cpuset.cpus
	if cpusetCPUs, err := readFile(cgroupPath, "cpuset.cpus"); err == nil {
		cpus := strings.TrimSpace(cpusetCPUs)
		count := countCPUs(cpus)
		e.gauge(cpuCpusDesc, float64(count), labels...)
	}

	// Read io.stat (cgroup v2), whose devices the io controller of the
	// cgroup accounts, and pids.current. A cgroup whose controller is not
	// enabled has neither file, and no series rather than a zero.
	if ioStat, err := readFile(cgroupPath, "io.stat"); err == nil {
		parseIOStat(e, ioStat, labels...)
	}
	if pids, err := readFile(cgroupPath, "pids.current"); err == nil {
		if val, err := parseUint(pids); err == nil {
			e.gauge(tasksDesc, float64(val), labels...)
		}
	}

	// Read io.weight (for cgroup v2) or blkio.weight (for cgroup v1)
	if ioWeight, err := readFile(cgroupPath, "io.weight"); err == nil {
		if val, err := parseIOWeight(ioWeight); err == nil {
			e.gauge(blkioWeightDesc, float64(val), labels...)
		}
	} else if blkioWeight, err := readFile(cgroupPath, "blkio.weight"); err == nil {
		if val, err := parseUint(blkioWeight); err == nil {
			e.gauge(blkioWeightDesc, float64(val), labels...)
		}
	}

	return memoryUtilization
}

func parseCPUStat(e *emitter, content string, labels ...string) {
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
			e.gauge(cpuUsageDesc, float64(statValue), labels...)
		case "user_usec":
			e.gauge(cpuUserUsageDesc, float64(statValue), labels...)
		case "system_usec":
			e.gauge(cpuSystemUsageDesc, float64(statValue), labels...)
		default:
			// Other CPU stats
			e.gauge(cpuStatDesc, float64(statValue), append(labels, statName)...)
		}
	}
}

// parseIOStat sums the io.stat lines, "<major>:<minor> rbytes=<n> wbytes=<n>
// rios=<n> wios=<n> ...", over the devices no other device is stacked on.
//
// An io to a device mapper, md or multipath device is accounted to it and to
// the devices under it too: summing them all would count it twice. The
// devices a stacked device holds are left out, as the holders directory of
// the device, or of one of its partitions, names them. The stacks the kernel
// does not record there, drbd over its backing device, a loop device over a
// file, are counted at both levels.
func parseIOStat(e *emitter, content string, labels ...string) {
	var rbytes, wbytes, rios, wios uint64
	for _, line := range strings.Split(content, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || e.isStacked(fields[0]) {
			continue
		}
		for _, field := range fields[1:] {
			k, v, ok := strings.Cut(field, "=")
			if !ok {
				continue
			}
			n, err := parseUint(v)
			if err != nil {
				continue
			}
			switch k {
			case "rbytes":
				rbytes += n
			case "wbytes":
				wbytes += n
			case "rios":
				rios += n
			case "wios":
				wios += n
			}
		}
	}
	e.gauge(ioReadBytesDesc, float64(rbytes), labels...)
	e.gauge(ioWriteBytesDesc, float64(wbytes), labels...)
	e.gauge(ioReadIOsDesc, float64(rios), labels...)
	e.gauge(ioWriteIOsDesc, float64(wios), labels...)
}

// isStacked says another block device is stacked on the device dev, a
// "<major>:<minor>": its holders directory, or the one of a partition of it,
// is not empty.
func (e *emitter) isStacked(dev string) bool {
	if v, ok := e.stacked[dev]; ok {
		return v
	}
	if e.stacked == nil {
		e.stacked = make(map[string]bool)
	}
	dir := filepath.Join(BlockRoot, dev)
	v := hasEntries(filepath.Join(dir, "holders"))
	if !v {
		entries, _ := os.ReadDir(dir)
		for _, entry := range entries {
			if _, err := os.Stat(filepath.Join(dir, entry.Name(), "partition")); err != nil {
				continue
			}
			if hasEntries(filepath.Join(dir, entry.Name(), "holders")) {
				v = true
				break
			}
		}
	}
	e.stacked[dev] = v
	return v
}

func hasEntries(dir string) bool {
	entries, err := os.ReadDir(dir)
	return err == nil && len(entries) > 0
}

func parseMemoryStat(e *emitter, content string, labels ...string) {
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
			e.gauge(memoryStatPagesDesc, float64(statValue), append(labels, statName)...)
		} else {
			e.gauge(memoryStatDesc, float64(statValue), append(labels, statName)...)
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
