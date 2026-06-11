// Package pgmetrics provides Prometheus metrics for cgroup resource usage
// under the opensvc.slice hierarchy.
//
// It exposes cgroup metrics (CPU, memory, etc.) for all cgroups created by
// the pg_* object keywords.
package pgmetrics

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
	"github.com/opensvc/om3/v3/util/systemd"
	"github.com/prometheus/client_golang/prometheus"
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
			Help:      "Memory max limit in bytes for the cgroup (0 = unlimited)",
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
			Help:      "CPU shares for the cgroup",
		},
		[]string{"namespace", "path"},
	)

	// pgCgroupCPUQuota reports CPU quota for each cgroup
	pgCgroupCPUQuota = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "opensvc",
			Subsystem: "pg",
			Name:      "cgroup_cpu_quota",
			Help:      "CPU quota for the cgroup",
		},
		[]string{"namespace", "path"},
	)

	// pgCgroupCPUPeriod reports CPU period for each cgroup
	pgCgroupCPUPeriod = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "opensvc",
			Subsystem: "pg",
			Name:      "cgroup_cpu_period",
			Help:      "CPU period for the cgroup",
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
			Help:      "Block IO weight for the cgroup",
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

// Manager manages the collection and reporting of cgroup metrics
type Manager struct {
	ctx       context.Context
	cancel    context.CancelFunc
	log       *plog.Logger
	localhost string
	sub       *pubsub.Subscription
	subQS     pubsub.QueueSizer

	wg sync.WaitGroup
}

// New creates a new pgmetrics manager with the specified queue size
func New(subQS pubsub.QueueSizer) *Manager {
	localhost := hostname.Hostname()
	return &Manager{
		localhost: localhost,
		subQS:     subQS,
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
	return nil
}

func (m *Manager) registerMetrics() {
	prometheus.MustRegister(
		pgCgroupCPUUsage,
		pgCgroupCPUUserUsage,
		pgCgroupCPUSystemUsage,
		pgCgroupMemoryCurrent,
		pgCgroupMemoryMax,
		pgCgroupMemoryStat,
		pgCgroupMemoryStatPages,
		pgCgroupCPUStat,
		pgCgroupCPUShares,
		pgCgroupCPUQuota,
		pgCgroupCPUPeriod,
		pgCgroupCPUCpus,
		pgCgroupBlkioWeight,
		pgCgroupExists,
	)
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
	for _, objPath := range objectPaths {
		// Forge the expected cgroup path from the object path
		cgroupPath := forgeCgroupPath(objPath)

		// Check if the cgroup path exists
		if _, err := os.Stat(cgroupPath); os.IsNotExist(err) {
			// Cgroup doesn't exist, skip this object
			continue
		}

		// Set the exists metric
		pgCgroupExists.WithLabelValues(objPath.Namespace, objPath.String()).Set(1)

		// Collect all metrics for this cgroup
		m.collectCgroupMetrics(cgroupPath, objPath.Namespace, objPath.String())
	}
}

func (m *Manager) collectCgroupMetrics(cgroupPath, namespace, objPath string) {
	// Read cpu.stat
	if cpuStat, err := readFile(cgroupPath, "cpu.stat"); err == nil {
		parseCPUStat(cpuStat, namespace, objPath)
	}

	// Read memory.current
	if memCurrent, err := readFile(cgroupPath, "memory.current"); err == nil {
		if val, err := parseUint(memCurrent); err == nil {
			pgCgroupMemoryCurrent.WithLabelValues(namespace, objPath).Set(float64(val))
		}
	}

	// Read memory.max
	if memMax, err := readFile(cgroupPath, "memory.max"); err == nil {
		if val, err := parseUint(memMax); err == nil {
			pgCgroupMemoryMax.WithLabelValues(namespace, objPath).Set(float64(val))
		}
	}

	// Read memory.stat
	if memStat, err := readFile(cgroupPath, "memory.stat"); err == nil {
		parseMemoryStat(memStat, namespace, objPath)
	}

	// Read cpu.shares
	if cpuShares, err := readFile(cgroupPath, "cpu.shares"); err == nil {
		if val, err := parseUint(cpuShares); err == nil {
			pgCgroupCPUShares.WithLabelValues(namespace, objPath).Set(float64(val))
		}
	}

	// Read cpu.cfs_quota_us
	if cpuQuota, err := readFile(cgroupPath, "cpu.cfs_quota_us"); err == nil {
		if val, err := parseInt(cpuQuota); err == nil {
			pgCgroupCPUQuota.WithLabelValues(namespace, objPath).Set(float64(val))
		}
	}

	// Read cpu.cfs_period_us
	if cpuPeriod, err := readFile(cgroupPath, "cpu.cfs_period_us"); err == nil {
		if val, err := parseUint(cpuPeriod); err == nil {
			pgCgroupCPUPeriod.WithLabelValues(namespace, objPath).Set(float64(val))
		}
	}

	// Read cpuset.cpus
	if cpusetCPUs, err := readFile(cgroupPath, "cpuset.cpus"); err == nil {
		cpus := strings.TrimSpace(cpusetCPUs)
		count := countCPUs(cpus)
		pgCgroupCPUCpus.WithLabelValues(namespace, objPath).Set(float64(count))
	}

	// Read io.weight (for cgroup v2) or blkio.weight (for cgroup v1)
	if blkioWeight, err := readFile(cgroupPath, "io.weight"); err == nil {
		if val, err := parseUint(blkioWeight); err == nil {
			pgCgroupBlkioWeight.WithLabelValues(namespace, objPath).Set(float64(val))
		}
	} else if blkioWeight, err := readFile(cgroupPath, "blkio.weight"); err == nil {
		if val, err := parseUint(blkioWeight); err == nil {
			pgCgroupBlkioWeight.WithLabelValues(namespace, objPath).Set(float64(val))
		}
	}
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
		"workingset_refault_anon":    true,
		"workingset_refault_file":    true,
		"workingset_activate_anon":    true,
		"workingset_activate_file":    true,
		"workingset_restore_anon":    true,
		"workingset_restore_file":    true,
		"workingset_nodereclaim":     true,
		"pgscan":                    true,
		"pgsteal":                   true,
		"pgscan_kswapd":             true,
		"pgscan_direct":             true,
		"pgscan_khugepaged":         true,
		"pgsteal_kswapd":            true,
		"pgsteal_direct":            true,
		"pgsteal_khugepaged":        true,
		"pgfault":                  true,
		"pgmajfault":                true,
		"pgrefill":                  true,
		"pgactivate":                true,
		"pgdeactivate":              true,
		"pglazyfree":                true,
		"pglazyfreed":               true,
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
