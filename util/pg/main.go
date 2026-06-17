package pg

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/xmap"
)

type (
	Config struct {
		ID            string
		CPUs          string
		Mems          string
		CPUShares     string
		CPUQuota      string
		MemOOMControl string
		MemLimit      string
		VMemLimit     string
		MemSwappiness string
		BlockIOWeight string
		applied       bool
		log           *plog.Logger
	}
	Mgr struct {
		configs map[string]*Config
	}
	CPUQuota string
	key      int
)

// WithLogger sets the logger for this Config and returns itself for chaining.
func (c *Config) WithLogger(l *plog.Logger) *Config {
	c.log = l
	return c
}

var mgrKey key = 0

func NewContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, mgrKey, &Mgr{
		configs: make(map[string]*Config),
	})
}

func FromContext(ctx context.Context) *Mgr {
	v := ctx.Value(mgrKey)
	if v == nil {
		return nil
	}
	return v.(*Mgr)
}

func (m *Mgr) Register(c *Config) {
	if c == nil {
		return
	}
	if _, ok := m.configs[c.ID]; ok {
		// Don't reset the "applied" bool if the config is registered again.
		// We don't need to handle in-run config changes.
		return
	}
	m.configs[c.ID] = c
}

// ApplyConfigs applies all registered pg configs in order (base to leaf).
// Each config is applied only if not already applied.
func (m *Mgr) ApplyConfigs() error {
	var errs error
	ids := xmap.Keys(m.configs)
	sort.Strings(ids)
	for _, id := range ids {
		c := m.configs[id]
		if _, err := c.ApplyOnce(); err != nil {
			errs = errors.Join(errs, err)
		}
	}
	return errs
}

func (m *Mgr) Clean() {
	// Clean in reverse order (LIFO)
	ids := xmap.Keys(m.configs)
	sort.Strings(ids)
	for i := len(ids) - 1; i >= 0; i-- {
		id := ids[i]
		m.configs[id].Clean()
	}
}

func UnifiedPath() string {
	mnt := "/sys/fs/cgroup"
	_, err := os.Stat(mnt + "/cgroup.procs")
	if err == nil {
		return mnt
	}
	return mnt + "/unified"
}

// ApplyOnce applies the pg configuration if it hasn't been applied already.
// Returns true if the config was applied, false if it was already applied.
func (c *Config) ApplyOnce() (bool, error) {
	if c == nil {
		return false, fmt.Errorf("no pg config")
	}
	if c.applied {
		return false, nil
	}
	created, err := c.ApplyProc(0)
	if err == nil {
		c.applied = true
		// Log at info level
		if c.log != nil {
			configStr := c.String()
			if strings.Contains(configStr, "=") {
				c.log.Infof("applied %s", configStr)
			} else if created {
				c.log.Infof("created %s", configStr)
			} else {
				c.log.Debugf("pg already exists: %s", configStr)
			}
		}
	}
	return err == nil, err
}

// Clean removes the pg configuration if it was applied.
func (c *Config) Clean() (bool, error) {
	if c == nil || !c.applied {
		return false, nil
	}
	c.applied = false
	changed, err := c.Delete()
	if changed && c.log != nil {
		c.log.Debugf("remove pg %s", c.ID)
	}
	return changed, err
}

// Apply is a convenience method for compatibility.
// Use (*Config).Apply() for state-tracking apply.
func (c Config) Apply() error {
	_, err := c.ApplyProc(os.Getpid())
	return err
}

func (c Config) String() string {
	buff := "pg " + c.ID
	l := make([]string, 0)
	if c.CPUs != "" {
		l = append(l, "cpus="+c.CPUs)
	}
	if c.Mems != "" {
		l = append(l, "mems="+c.Mems)
	}
	if c.CPUShares != "" {
		l = append(l, "cpu_shares="+c.CPUShares)
	}
	if c.CPUQuota != "" {
		l = append(l, "cpu_quota="+c.CPUQuota)
	}
	if c.MemOOMControl != "" {
		l = append(l, "mem_oom_control="+c.MemOOMControl)
	}
	if c.MemLimit != "" {
		l = append(l, "mem_limit="+c.MemLimit)
	}
	if c.VMemLimit != "" {
		l = append(l, "vmem_limit="+c.VMemLimit)
	}
	if c.MemSwappiness != "" {
		l = append(l, "mem_swappiness="+c.MemSwappiness)
	}
	if c.BlockIOWeight != "" {
		l = append(l, "blkioweight="+c.BlockIOWeight)
	}
	if len(l) == 0 {
		return buff
	}
	return buff + ": " + strings.Join(l, " ")
}

// Convert converts, for a 100us period and 4 cpu threads,
// * 100%@all => 100000 100000
// * 50% => 50000 100000
// * 10%@2 => 5000 100000
func (t CPUQuota) Convert(period uint64) (int64, error) {
	maxCpus := runtime.NumCPU()
	invalidFmtError := "invalid cpu quota format: %s (accepted expressions: 1000, 50%%@all, 10%%@2)"
	parsePct := func(s string) (int, error) {
		if strings.HasSuffix(s, "%") {
			s = strings.TrimRight(s, "%")
		}
		return strconv.Atoi(s)
	}
	parseCpus := func(s string) (int, error) {
		if (s == "all") || (s == "") {
			return maxCpus, nil
		} else if cpus, err := strconv.Atoi(s); err != nil {
			return 0, fmt.Errorf(invalidFmtError+":%w", t, err)
		} else if cpus > maxCpus {
			return maxCpus, nil
		} else {
			return cpus, nil
		}
	}

	l := strings.Split(string(t), "@")
	var cpusString string

	switch len(l) {
	case 1:
		cpusString = "1"
	case 2:
		cpusString = l[1]
	default:
		return 0, fmt.Errorf(invalidFmtError, t)
	}

	var (
		cpus, pct int
		err       error
	)
	if cpus, err = parseCpus(cpusString); err != nil {
		return 0, fmt.Errorf(invalidFmtError+":%w", t, err)
	}
	if pct, err = parsePct(l[0]); err != nil {
		return 0, fmt.Errorf(invalidFmtError+":%w", t, err)
	}
	return int64(pct) * int64(period) * int64(cpus) / int64(maxCpus) / 100, nil
}
