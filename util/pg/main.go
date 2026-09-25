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

		// Delegation, when set, places the group under a subtree of the
		// hierarchy delegated to an unprivileged user, rather than at the
		// root. The ID stays the one the configuration gives: it is what
		// orders the groups and what a reset is scoped by, and it is the
		// same group of the same object wherever it lives.
		Delegation *Delegation

		applied bool
		reset   bool
		log     *plog.Logger
	}

	// Delegation is a subtree of the unified hierarchy delegated to an
	// unprivileged user, the way systemd delegates user@<uid>.service to
	// the systemd instance of that user.
	//
	// A group made under it is owned by that user, so an engine running as
	// them can create its own groups inside it: that is what lets a rootless
	// container be placed in the group om caps. The cappings themselves are
	// written by om, in files the user is not given, so they hold against
	// the user as they hold against the container.
	Delegation struct {
		// Root is the delegated subtree, relative to the unified mount,
		// as /user.slice/user-1001.slice/user@1001.service.
		Root string

		// UID and GID own the groups made under Root.
		UID int
		GID int
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

// Path is where the group lives in the unified hierarchy: its ID, under the
// delegated subtree when there is one.
func (c Config) Path() string {
	if c.Delegation == nil {
		return c.ID
	}
	return c.Delegation.Root + c.ID
}

// Delegated returns the same group, placed under a delegated subtree.
func (c Config) Delegated(d Delegation) *Config {
	c.Delegation = &d
	c.applied = false
	c.reset = false
	return &c
}

// Register adds a group to those an action applies, keyed by where it lives:
// the same group delegated to a user is another group of the hierarchy, and
// is made and capped on its own.
func (m *Mgr) Register(c *Config) {
	if c == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.configs[c.Path()]; ok {
		// Don't reset the "applied" bool if the config is registered again.
		// We don't need to handle in-run config changes.
		return
	}
	m.configs[c.Path()] = c
}

// Configs returns the registered groups, sorted by where they live.
func (m *Mgr) Configs() []*Config {
	m.mu.Lock()
	defer m.mu.Unlock()
	l := make([]*Config, 0, len(m.configs))
	for _, c := range m.configs {
		l = append(l, c)
	}
	sort.Slice(l, func(i, j int) bool { return l[i].Path() < l[j].Path() })
	return l
}

// Ancestors returns the registered groups id is nested in, parents first.
//
// It is what a group moved under a delegated subtree carries along: the
// cappings of the namespace and the object are written on their groups, and
// a group placed elsewhere is only capped by them if they are made above it
// there too.
func (m *Mgr) Ancestors(id string) []*Config {
	m.mu.Lock()
	defer m.mu.Unlock()
	l := make([]*Config, 0)
	for _, c := range m.configs {
		if c.Delegation != nil {
			continue
		}
		if c.ID != id && strings.HasPrefix(id, c.ID+"/") {
			l = append(l, c)
		}
	}
	sort.Slice(l, func(i, j int) bool { return l[i].ID < l[j].ID })
	return l
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
	paths := xmap.Keys(m.configs)
	sort.Strings(paths)
	for _, path := range paths {
		c := m.configs[path]
		if c.ID != m.resetRoot && !strings.HasPrefix(c.ID, m.resetRoot+"/") {
			continue
		}
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
			c.log.Infof("reset pg %s", path)
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
		c.log.Debugf("remove pg %s", c.Path())
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
	buff := "pg " + c.Path()
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
