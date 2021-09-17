package pg

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	cgroups "github.com/containerd/cgroups"
	cgroupsv2 "github.com/containerd/cgroups/v2"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"opensvc.com/opensvc/util/converters"
	"opensvc.com/opensvc/util/sizeconv"
	"opensvc.com/opensvc/util/xmap"
)

type (
	Config struct {
		ID            string
		Cpus          string
		Mems          string
		CpuShares     string
		CpuQuota      string
		MemOOMControl string
		MemLimit      string
		VMemLimit     string
		MemSwappiness string
		BlkioWeight   string
	}
	CpuQuota string
	key      int
	State    int
	entry    struct {
		State  State
		Config *Config
	}
	run struct {
		Err     error
		Config  *Config
		Changed bool
	}
	Mgr map[string]*entry
)

const (
	Init State = iota
	Applied
	Deleted
)

var (
	mgrKey      key    = 0
	UnifiedPath string = "/sys/fs/cgroup/unified"
)

func (e entry) NewRun() *run {
	r := run{Config: e.Config}
	return &r
}

func NewContext(ctx context.Context) context.Context {
	t := make(Mgr)
	return context.WithValue(ctx, mgrKey, &t)
}

func FromContext(ctx context.Context) *Mgr {
	v := ctx.Value(mgrKey)
	if v == nil {
		return nil
	}
	return v.(*Mgr)
}

func (t Mgr) Register(c *Config) {
	if c == nil {
		return
	}
	if _, ok := t[c.ID]; ok {
		return
	}
	t[c.ID] = &entry{
		Config: c,
	}
}

// Chain return the list of self and parents from closest to farthest
func RevChain(id string) []string {
	chain := make([]string, 0)
	for {
		chain = append(chain, id)
		nid := filepath.Dir(id)
		if nid == id {
			break
		}
		id = nid
	}
	return chain
}

func Chain(id string) []string {
	l := RevChain(id)
	sort.Sort(sort.StringSlice(l))
	return l
}

func (t Mgr) IDs() []string {
	return xmap.Keys(t)
}

func (t Mgr) RevIDs() []string {
	l := t.IDs()
	sort.Sort(sort.Reverse(sort.StringSlice(l)))
	return l
}

func (t Mgr) Clean() []run {
	runs := make([]run, 0)
	for _, p := range t.RevIDs() {
		e, ok := t[p]
		if !ok {
			continue
		}
		r := e.NewRun()
		switch e.State {
		case Init, Applied:
			if r.Changed, r.Err = e.Config.Delete(); r.Changed {
				e.State = Deleted
			}
		default:
			r.Changed = false
		}
		runs = append(runs, *r)
	}
	return runs
}

func (e *entry) Run() *run {
	r := e.NewRun()
	if e.Config == nil {
		r.Err = fmt.Errorf("no pg config")
		r.Changed = false
		return r
	}
	switch e.State {
	case Init, Deleted:
		e.State = Applied
		r.Err = e.Config.ApplyNoProc()
		r.Changed = true
	case Applied:
		r.Changed = false
		return r
	}
	return r
}

func (t Mgr) Apply(id string) []run {
	runs := make([]run, 0)
	for _, p := range Chain(id) {
		if e, ok := t[p]; ok {
			r := e.Run()
			runs = append(runs, *r)
		}
	}
	return runs
}

func (c Config) String() string {
	buff := "pg " + c.ID
	l := make([]string, 0)
	if c.Cpus != "" {
		l = append(l, "cpus="+c.Cpus)
	}
	if c.Mems != "" {
		l = append(l, "mems="+c.Mems)
	}
	if c.CpuShares != "" {
		l = append(l, "cpu_shares="+c.CpuShares)
	}
	if c.CpuQuota != "" {
		l = append(l, "cpu_quota="+c.CpuQuota)
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
	if c.BlkioWeight != "" {
		l = append(l, "blkioweight="+c.BlkioWeight)
	}
	if len(l) == 0 {
		return buff
	}
	return buff + ": " + strings.Join(l, " ")
}

//
// Convert converts, for a 100us period and 4 cpu threads,
// * 100%@all => 400000 100000
// * 50% => 50000 100000
// * 50%@3 => 150000 100000
//
func (t CpuQuota) Convert(period uint64) (int64, error) {
	maxCpus := runtime.NumCPU()
	parsePct := func(s string) (int, error) {
		if strings.HasSuffix(s, "%") {
			s = strings.TrimRight(s, "%")
		}
		return strconv.Atoi(s)
	}
	parseCpus := func(s string) (int, error) {
		if s == "all" {
			return maxCpus, nil
		} else if cpus, err := strconv.Atoi(s); err != nil {
			return 0, errors.Wrapf(err, "invalid cpu quota format: %s (accepted expressions: 1000, 50%@all, 10%@2)", t)
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
		return 0, fmt.Errorf("invalid cpu quota format: %s (accepted expressions: 1000, 50%@all, 10%@2)", t)
	}

	var (
		cpus, pct int
		err       error
	)
	if cpus, err = parseCpus(cpusString); err != nil {
		return 0, errors.Wrapf(err, "invalid cpu quota format: %s (accepted expressions: 1000, 50%@all, 10%@2)", t)
	}
	if pct, err = parsePct(l[0]); err != nil {
		return 0, errors.Wrapf(err, "invalid cpu quota format: %s (accepted expressions: 1000, 50%@all, 10%@2)", t)
	}
	return int64(pct) * int64(cpus) * int64(period) / 100, nil
}

// ApplyNoProc creates the cgroup, set caps, but does not add a process
func (c Config) ApplyNoProc() error {
	return c.ApplyProc(0)
}

// Apply creates the cgroup, set caps, and add the running process
func (c Config) Apply() error {
	return c.ApplyProc(os.Getpid())
}

// ApplyProc creates the cgroup, set caps, and add the specified process
func (c Config) ApplyProc(pid int) error {
	if c.ID == "" {
		return fmt.Errorf("Config Path is mandatory")
	}
	r := specs.LinuxResources{
		CPU:     &specs.LinuxCPU{},
		Memory:  &specs.LinuxMemory{},
		BlockIO: &specs.LinuxBlockIO{},
	}
	if n, err := sizeconv.FromSize(c.CpuShares); err == nil {
		shares := uint64(n)
		r.CPU.Shares = &shares
	}
	if c.Cpus != "" {
		r.CPU.Cpus = c.Cpus
	}
	if c.Mems != "" {
		r.CPU.Mems = c.Mems
	}
	period := uint64(100000)
	if quota, err := CpuQuota(c.CpuQuota).Convert(period); err == nil {
		r.CPU.Period = &period
		r.CPU.Quota = &quota
	}
	var (
		memLimit int64
		memError error
	)
	if memLimit, memError = strconv.ParseInt(c.MemLimit, 10, 64); memError == nil {
		r.Memory.Limit = &memLimit
	}
	if n, err := strconv.ParseInt(c.VMemLimit, 10, 64); err == nil {
		swap := n - memLimit
		r.Memory.Swap = &swap
	}
	if n, err := strconv.ParseUint(c.MemSwappiness, 10, 64); err == nil {
		r.Memory.Swappiness = &n
	}
	if n, err := converters.Bool.Convert(c.MemOOMControl); err == nil {
		disable := n.(bool)
		r.Memory.DisableOOMKiller = &disable
	}
	if n, err := strconv.ParseUint(c.BlkioWeight, 10, 16); err == nil {
		weight := uint16(n)
		r.BlockIO.Weight = &weight
	}

	control, err := cgroupsv2.NewManager(UnifiedPath, c.ID, cgroupsv2.ToResources(&r))
	if err == nil {
		if pid == 0 {
			return nil
		}
		if err := control.AddProc(uint64(pid)); err != nil {
			return errors.Wrapf(err, "add pid to pg %s", c.ID)
		}
	} else {
		control, err := cgroups.New(cgroups.V1, cgroups.StaticPath(c.ID), &r)
		if err != nil {
			return errors.Wrapf(err, "cnf pg %s", c.ID)
		}
		if pid == 0 {
			return nil
		}
		if err := control.Add(cgroups.Process{Pid: pid}); err != nil {
			return errors.Wrapf(err, "add pid to pg %s", c.ID)
		}
	}
	return nil
}

func (c Config) Delete() (bool, error) {
	var changed bool
	if ch, err := c.deleteV1(); err != nil {
		return changed, err
	} else {
		changed = changed || ch
	}
	if ch, err := c.deleteV2(); err != nil {
		return changed, err
	} else {
		changed = changed || ch
	}
	return changed, nil
}

func (c Config) deleteV2() (bool, error) {
	control, err := cgroupsv2.LoadManager(UnifiedPath, c.ID)
	if err != nil {
		// doesn't verify path existance
		return false, nil
	}
	if _, err := control.Controllers(); err != nil {
		// path does not exist, delete not needed
		return false, nil
	}
	if err := control.Delete(); err != nil {
		return false, err
	}
	return true, nil
}

func (c Config) deleteV1() (bool, error) {
	control, err := cgroups.Load(cgroups.V1, cgroups.StaticPath(c.ID))
	if err != nil {
		return false, nil
	}
	if err := control.Delete(); err != nil {
		return false, err
	}
	return true, nil
}
