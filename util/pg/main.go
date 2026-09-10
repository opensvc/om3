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
	"sync"

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
		reset         bool
		log           *plog.Logger
	}
	Mgr struct {
		mu        sync.Mutex
		configs   map[string]*Config
		resetRoot string
	}
	CPUQuota string
	key      int
)

// DefaultValue is the pg_* keyword value asking for the capping it names to
// be put back where a node that never capped anything leaves it.
//
// Removing a keyword leaves the capping alone, on purpose: what om wrote
// stays, and so does what anything else wrote. That leaves no way of lifting
// a cap, which this value is. It is a value and not a command because the
// configuration is what every node of the cluster converges to: a node
// provisioned tomorrow, or a peer taking the object over, has to arrive at
// the same capping as the node the reset was asked on.
const DefaultValue = "default"

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
	m.mu.Lock()
	defer m.mu.Unlock()
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
	m.mu.Lock()
	defer m.mu.Unlock()
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

// SetResetRoot says which of the registered groups an action may lift the
// capping of: that one, and the ones under it.
//
// It is what keeps an object from uncapping what it does not own. The groups
// registered by an action reach above the object, up to the slice of the
// namespace and the slice holding every object of the node, because a child
// cannot be capped before its parents exist. Those are the namespace's to
// lift, not this object's.
func (m *Mgr) SetResetRoot(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resetRoot = id
}

// ResetConfigs lifts the capping of the registered groups at and under the
// reset root, base to leaf, each one once.
//
// The order matters the same way it does when capping: a parent has to be
// uncapped before a child, or the child stays capped by what the parent still
// holds. Sorting the ids gives that order, a parent being a prefix of its
// children.
//
// It is called once per resource of the walk, as applying is, and lifts on
// each call only what the calls before it did not: a resource registers its
// group and the groups registered so far are already lifted.
func (m *Mgr) ResetConfigs() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.resetRoot == "" {
		return nil
	}
	var errs error
	ids := xmap.Keys(m.configs)
	sort.Strings(ids)
	for _, id := range ids {
		if id != m.resetRoot && !strings.HasPrefix(id, m.resetRoot+"/") {
			continue
		}
		c := m.configs[id]
		if c.reset {
			continue
		}
		uncapped := c.Uncapped()
		if _, err := uncapped.ApplyProc(0); err != nil {
			errs = errors.Join(errs, err)
			continue
		}
		c.reset = true
		if c.log != nil {
			c.log.Infof("reset pg %s", id)
		}
	}
	return errs
}

func (m *Mgr) Clean() {
	m.mu.Lock()
	defer m.mu.Unlock()
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
// Uncapped returns the same group, with every capping the unified hierarchy
// has a file for asked to go back where the kernel leaves it.
//
// It is what "pg reset" applies. The keywords a configuration carries are not
// read: the point of the command is to lift cappings the configuration does
// not know about, left by an older agent, by systemd, or by hand.
func (c Config) Uncapped() Config {
	c.CPUs = DefaultValue
	c.Mems = DefaultValue
	c.CPUShares = DefaultValue
	c.CPUQuota = DefaultValue
	c.MemLimit = DefaultValue
	c.VMemLimit = DefaultValue
	c.BlockIOWeight = DefaultValue
	c.applied = false
	return c
}

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

// String is what an applied group logs, so it names the cappings that were
// applied and not the ones this node has no file for.
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
	if c.MemOOMControl != "" && !isIgnored("pg_mem_oom_control") {
		l = append(l, "mem_oom_control="+c.MemOOMControl)
	}
	if c.MemLimit != "" {
		l = append(l, "mem_limit="+c.MemLimit)
	}
	if c.VMemLimit != "" {
		l = append(l, "vmem_limit="+c.VMemLimit)
	}
	if c.MemSwappiness != "" && !isIgnored("pg_mem_swappiness") {
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

// Convert returns the cpu.max quota of a pg_cpu_quota expression, for the
// period it is given.
//
// The percentage is of the cpus the expression names, one when it names none,
// so for a 100000 period on a node having 4 cpu threads:
//
//	100%@all => 400000, the four of them
//	50%      => 50000, half of one
//	10%@2    => 20000, a tenth of two
func (t CPUQuota) Convert(period uint64) (int64, error) {
	maxCpus := runtime.NumCPU()
	invalidFmtError := "invalid cpu quota format: %s (accepted expressions: 50%%, 50%%@all, 10%%@2)"
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
	// The percentage is of the cpus named, and of those alone. Dividing by
	// the cpus the node has as well gave every group a quota that was the
	// documented one divided by the thread count of the node: a "50%" asking
	// for half a cpu got an eighth of one on an 8 thread node.
	return int64(pct) * int64(period) * int64(cpus) / 100, nil
}
