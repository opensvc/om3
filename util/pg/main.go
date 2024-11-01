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

	"github.com/opensvc/om3/util/xmap"
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
	}
	CPUQuota string
	key      int
	State    int
	entry    struct {
		State  State
		Config *Config
	}
	Run struct {
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
	mgrKey key = 0
)

func UnifiedPath() string {
	mnt := "/sys/fs/cgroup"
	_, err := os.Stat(mnt + "/cgroup.procs")
	if err == nil {
		return mnt
	}
	return mnt + "/unified"
}

func (e entry) NewRun() *Run {
	r := Run{Config: e.Config}
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

// RevChain returns the list of self and parents from closest to farthest
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

func (t Mgr) Clean() []Run {
	runs := make([]Run, 0)
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

func (e *entry) Run() *Run {
	r := e.NewRun()
	if e.Config == nil {
		r.Err = fmt.Errorf("no pg config")
		r.Changed = false
		return r
	}
	if !e.Config.needApply() {
		r.Err = fmt.Errorf("empty pg config ")
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

func (t Mgr) Apply(id string) []Run {
	runs := make([]Run, 0)
	for _, p := range Chain(id) {
		if e, ok := t[p]; ok {
			r := e.Run()
			runs = append(runs, *r)
		}
	}
	return runs
}

func (c Config) needApply() bool {
	if c.CPUs != "" {
		return true
	}
	if c.Mems != "" {
		return true
	}
	if c.CPUShares != "" {
		return true
	}
	if c.CPUQuota != "" {
		return true
	}
	if c.MemOOMControl != "" {
		return true
	}
	if c.MemLimit != "" {
		return true
	}
	if c.VMemLimit != "" {
		return true
	}
	if c.MemSwappiness != "" {
		return true
	}
	if c.BlockIOWeight != "" {
		return true
	}
	return false
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

// ApplyNoProc creates the cgroup, set caps, but does not add a process
func (c Config) ApplyNoProc() error {
	return c.ApplyProc(0)
}

// Apply creates the cgroup, set caps, and add the running process
func (c Config) Apply() error {
	return c.ApplyProc(os.Getpid())
}
