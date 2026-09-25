//go:build linux

package pg

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/containerd/cgroups"
	cgroupsv2 "github.com/containerd/cgroups/v2"
	"github.com/opencontainers/runtime-spec/specs-go"

	"github.com/opensvc/om3/v3/util/converters"
	"github.com/opensvc/om3/v3/util/sizeconv"
)

// unifiedWrite is a value to write in a file of the unified hierarchy.
type unifiedWrite struct {
	file  string
	value string
}

// unifiedResets says, for each pg_* keyword accepting DefaultValue, what an
// uncapped cgroup holds.
//
// The values are the ones a cgroup nobody has capped holds, read from a fresh
// one rather than assumed, and each was checked to be accepted written the way
// it reads back.
//
// pg_mem_swappiness and pg_mem_oom_control are absent because the unified
// hierarchy has no file for either: memory.swappiness and memory.oom_control
// are of the v1 hierarchy, and setting those keywords already does nothing
// here.
var unifiedResets = map[string]unifiedWrite{
	"pg_cpu_quota":    {"cpu.max", "max 100000"},
	"pg_cpu_shares":   {"cpu.weight", "100"},
	"pg_cpus":         {"cpuset.cpus", ""},
	"pg_mems":         {"cpuset.mems", ""},
	"pg_mem_limit":    {"memory.max", "max"},
	"pg_vmem_limit":   {"memory.swap.max", "max"},
	"pg_blkio_weight": {"io.weight", "default 100"},
}

// applyUnifiedWrites writes what the cgroup manager cannot say.
//
// It cannot say a reset: it skips an empty cpuset, which is what clearing one
// is, and it writes a memory limit as a number, where lifting one is the word
// "max". And it cannot say a block io weight: it writes io.bfq.weight, a file
// only a kernel running the bfq scheduler has, where the v2 agent writes
// io.weight, which the io controller always has.
func applyUnifiedWrites(id string, writes []unifiedWrite) error {
	var errs error
	for _, write := range writes {
		path := filepath.Join(UnifiedPath(), id, write.file)
		if err := os.WriteFile(path, []byte(write.value), 0644); err != nil {
			errs = errors.Join(errs, fmt.Errorf("write %s: %w", write.file, err))
		}
	}
	return errs
}

// ApplyProc creates the cgroup, set caps, and add the specified process
//
// On the unified hierarchy every value is written where the kernel keeps it,
// and the manager is asked only to make the cgroup and to delegate the
// controllers to it. It converts an oci runtime spec, whose weights are of the
// v1 hierarchy, and rescales them: a cpu weight of 1024 reached the kernel as
// 39, and a block io weight went to io.bfq.weight, a file only a kernel
// running the bfq scheduler has. It also cannot say a reset: it skips an empty
// cpuset, which is what clearing one is, and writes a memory limit as a
// number, where lifting one is the word "max".
//
// The v1 hierarchy still goes through it, which is what it was written for.
//
// Which of the two a node holds is read from the mode of its cgroup mounts,
// not from the unified manager failing. Falling back on a failure confused a
// node having no unified hierarchy with a write that went wrong on one, and
// the second sent the apply down a v1 path that could only fail differently:
// that is how a missing io.bfq.weight was reported as a missing v1 mountpoint.
func (c Config) ApplyProc(pid int) (created bool, errs error) {
	if c.ID == "" {
		errs = fmt.Errorf("the pg config application requires a non empty pg id")
		return
	}

	unified := isUnified()
	writes := make([]unifiedWrite, 0)
	var hasReset bool
	reset := func(kw string) {
		hasReset = true
		writes = append(writes, unifiedResets[kw])
	}
	write := func(file, value string) {
		writes = append(writes, unifiedWrite{file, value})
	}

	// The oci spec is built as it was, for the v1 hierarchy alone.
	r := specs.LinuxResources{
		CPU:     &specs.LinuxCPU{},
		Memory:  &specs.LinuxMemory{},
		BlockIO: &specs.LinuxBlockIO{},
	}

	if c.CPUShares == DefaultValue {
		reset("pg_cpu_shares")
	} else if n, err := sizeconv.FromSize(c.CPUShares); err == nil {
		shares := uint64(n)
		r.CPU.Shares = &shares
		write("cpu.weight", strconv.FormatUint(shares, 10))
	}
	if c.CPUs == DefaultValue {
		reset("pg_cpus")
	} else if c.CPUs != "" {
		r.CPU.Cpus = c.CPUs
		write("cpuset.cpus", c.CPUs)
	}
	if c.Mems == DefaultValue {
		reset("pg_mems")
	} else if c.Mems != "" {
		r.CPU.Mems = c.Mems
		write("cpuset.mems", c.Mems)
	}
	if c.CPUQuota == DefaultValue {
		reset("pg_cpu_quota")
	} else if c.CPUQuota != "" {
		period := uint64(100000)
		if quota, err := CPUQuota(c.CPUQuota).Convert(period); err != nil {
			errs = errors.Join(errs, fmt.Errorf("pg_cpu_quota: %w", err))
		} else {
			r.CPU.Period = &period
			r.CPU.Quota = &quota
			write("cpu.max", fmt.Sprintf("%d %d", quota, period))
		}
	}
	if c.MemLimit == DefaultValue {
		reset("pg_mem_limit")
		if c.VMemLimit == DefaultValue {
			reset("pg_vmem_limit")
		}
	} else if c.MemLimit != "" {
		if n, err := sizeconv.FromSize(c.MemLimit); err != nil {
			errs = errors.Join(errs, fmt.Errorf("pg_mem_limit: %w", err))
		} else {
			r.Memory.Limit = &n
			write("memory.max", strconv.FormatInt(n, 10))
			if c.VMemLimit != "" {
				if v, err := sizeconv.FromSize(c.VMemLimit); err != nil {
					errs = errors.Join(errs, fmt.Errorf("pg_vmem_limit: %w", err))
				} else {
					swap := v - n
					r.Memory.Swap = &swap
					write("memory.swap.max", strconv.FormatInt(swap, 10))
				}
			}
		}
	}

	// pg_mem_swappiness and pg_mem_oom_control reach the v1 hierarchy only:
	// memory.swappiness and memory.oom_control are of that hierarchy, and the
	// v2 agent writes neither on the unified one either. Setting one on a
	// node holding the unified hierarchy caps nothing, so the group says so
	// rather than letting the operator believe it capped something.
	if unified && c.log != nil {
		for kw, value := range map[string]string{
			"pg_mem_swappiness":  c.MemSwappiness,
			"pg_mem_oom_control": c.MemOOMControl,
		} {
			if value == "" {
				continue
			}
			c.log.Warnf("%s is ignored: the unified cgroup hierarchy has no %s", kw, ignoredOnUnified[kw])
		}
	}
	if c.MemSwappiness != "" {
		if n, err := strconv.ParseUint(c.MemSwappiness, 10, 64); err != nil {
			errs = errors.Join(errs, fmt.Errorf("pg_mem_swapiness: %w", err))
		} else {
			r.Memory.Swappiness = &n
		}
	}
	if c.MemOOMControl != "" {
		if n, err := converters.Bool.Convert(c.MemOOMControl); err != nil {
			errs = errors.Join(errs, fmt.Errorf("pg_mem_oom_control: %w", err))
		} else {
			disable := n.(bool)
			r.Memory.DisableOOMKiller = &disable
		}
	}
	if c.BlockIOWeight == DefaultValue {
		reset("pg_blkio_weight")
	} else if c.BlockIOWeight != "" {
		if n, err := strconv.ParseUint(c.BlockIOWeight, 10, 16); err != nil {
			errs = errors.Join(errs, fmt.Errorf("pg_blkio_weight: %w", err))
		} else {
			weight := uint16(n)
			r.BlockIO.Weight = &weight
			write("io.weight", c.BlockIOWeight)
		}
	}

	if c.Delegation != nil {
		if !unified {
			errs = errors.Join(errs, fmt.Errorf("pg %s: a group delegated to a user needs the unified cgroup hierarchy", c.ID))
			return
		}
		var err error
		created, err = c.makeDelegated()
		if err != nil {
			errs = errors.Join(errs, err)
			return
		}
		errs = errors.Join(errs, c.applyDelegatedWrites(writes))
		errs = errors.Join(errs, c.setSystemdProperties())
		if pid != 0 {
			procs := filepath.Join(UnifiedPath(), c.Path(), "cgroup.procs")
			if err := os.WriteFile(procs, []byte(strconv.Itoa(pid)), 0644); err != nil {
				errs = errors.Join(errs, fmt.Errorf("add pid to pg %s: %w", c.Path(), err))
			}
		}
		return
	}

	if unified {
		control, err := cgroupsv2.NewManager(UnifiedPath(), c.ID, delegatedControllers())
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("new pg %s: %w", c.ID, err))
			return
		}
		if len(writes) > 0 {
			errs = errors.Join(errs, applyUnifiedWrites(c.ID, writes))
		}
		errs = errors.Join(errs, c.setSystemdProperties())
		if pid == 0 {
			// pass
		} else if err := control.AddProc(uint64(pid)); err != nil {
			errs = errors.Join(errs, fmt.Errorf("add pid to pg %s: %w", c.ID, err))
		}
		return
	}

	if hasReset {
		// The v1 hierarchy names these files otherwise and holds other
		// values in them, and none of that was read from a v1 node.
		errs = errors.Join(errs, fmt.Errorf("a pg keyword set to %q needs the unified cgroup hierarchy", DefaultValue))
	}
	control, err := cgroups.New(cgroups.V1, cgroups.StaticPath(c.ID), &r)
	if err != nil {
		errs = errors.Join(errs, fmt.Errorf("new pg %s: %w", c.ID, err))
	} else if pid == 0 {
		created = true
		// pass
	} else if err := control.Add(cgroups.Process{Pid: pid}); err != nil {
		created = true
		errs = errors.Join(errs, fmt.Errorf("add pid to pg %s: %w", c.ID, err))
	}
	return
}

// ignoredOnUnified is the keyword whose capping the unified hierarchy has no
// file for, and the file the v1 hierarchy keeps it in.
//
// The v2 agent writes neither on the unified hierarchy either.
var ignoredOnUnified = map[string]string{
	"pg_mem_swappiness":  "memory.swappiness",
	"pg_mem_oom_control": "memory.oom_control",
}

// isIgnored reports whether a keyword caps nothing on this node.
func isIgnored(kw string) bool {
	if !isUnified() {
		return false
	}
	_, ok := ignoredOnUnified[kw]
	return ok
}

// isUnified reports whether the node holds the unified cgroup hierarchy and
// nothing else.
//
// A hybrid node has both mounted, and its controllers are on the v1 one: the
// unified hierarchy is there with no controller delegated to it, so a value
// written under it would reach a file that does not exist. It is a v1 node as
// far as capping goes.
func isUnified() bool {
	return cgroups.Mode() == cgroups.Unified
}

// delegatedControllers is what the manager is handed to make the cgroup with.
//
// It carries no value, every one of them being written afterwards, and names
// the controllers by being made of them: the manager reads which to write in
// cgroup.subtree_control of the ancestors from which of these is not nil, and
// a cgroup whose controllers are not delegated to it has none of the files
// the values go in.
func delegatedControllers() *cgroupsv2.Resources {
	return &cgroupsv2.Resources{
		CPU:    &cgroupsv2.CPU{},
		Memory: &cgroupsv2.Memory{},
		IO:     &cgroupsv2.IO{},
	}
}

// delegationFiles are the files of a group its owner has to be given for the
// group to be delegated to them: they create groups in it, move processes
// between them, and enable controllers for them. The files holding the
// cappings are not among them, so the owner cannot lift what is written there.
var delegationFiles = []string{"cgroup.procs", "cgroup.subtree_control", "cgroup.threads"}

// delegatedControllerNames are the controllers a group delegated to a user is
// made with, when the subtree offers them: the ones the pg keywords write to.
var delegatedControllerNames = []string{"cpu", "cpuset", "io", "memory"}

// makeDelegated makes the group under its delegated subtree, and every group
// between the two, owned by the user the subtree is delegated to.
//
// The subtree itself must exist: it is the systemd instance of the user that
// makes it, and its absence says that instance is not running, which no group
// om could make would fix.
//
// Each group is given the controllers of the list its parent offers. A
// controller the subtree was not delegated, like io and cpuset under a default
// user@.service, cannot be given, and what is capped through it is reported
// when its file is found missing.
func (c Config) makeDelegated() (created bool, err error) {
	d := c.Delegation
	root := filepath.Join(UnifiedPath(), d.Root)
	if _, err := os.Stat(root); err != nil {
		return false, fmt.Errorf("pg %s: the delegated cgroup %s does not exist: the systemd instance of uid %d is not running", c.ID, d.Root, d.UID)
	}
	parent := root
	for _, name := range strings.Split(strings.Trim(c.ID, "/"), "/") {
		if name == "" {
			continue
		}
		if err := enableControllers(parent); err != nil {
			return created, fmt.Errorf("pg %s: %w", c.ID, err)
		}
		dir := filepath.Join(parent, name)
		if err := os.Mkdir(dir, 0755); err == nil {
			created = true
		} else if !errors.Is(err, os.ErrExist) {
			return created, fmt.Errorf("pg %s: %w", c.ID, err)
		}
		if err := chownDelegated(dir, d.UID, d.GID); err != nil {
			return created, fmt.Errorf("pg %s: %w", c.ID, err)
		}
		parent = dir
	}
	// The leaf is given its controllers too: the engine makes the group of
	// the container under it, and that group has the files of none of them
	// otherwise.
	if err := enableControllers(parent); err != nil {
		return created, fmt.Errorf("pg %s: %w", c.ID, err)
	}
	return created, nil
}

// enableControllers gives the children of a group the controllers of the list
// the group has.
func enableControllers(dir string) error {
	b, err := os.ReadFile(filepath.Join(dir, "cgroup.controllers"))
	if err != nil {
		return err
	}
	available := strings.Fields(string(b))
	b, err = os.ReadFile(filepath.Join(dir, "cgroup.subtree_control"))
	if err != nil {
		return err
	}
	enabled := strings.Fields(string(b))
	l := make([]string, 0)
	for _, name := range delegatedControllerNames {
		if slices.Contains(available, name) && !slices.Contains(enabled, name) {
			l = append(l, "+"+name)
		}
	}
	if len(l) == 0 {
		return nil
	}
	path := filepath.Join(dir, "cgroup.subtree_control")
	if err := os.WriteFile(path, []byte(strings.Join(l, " ")), 0644); err != nil {
		return fmt.Errorf("enable %s in %s: %w", strings.Join(l, " "), dir, err)
	}
	return nil
}

// chownDelegated gives a group, and the files delegating it, to its owner.
func chownDelegated(dir string, uid, gid int) error {
	if err := os.Chown(dir, uid, gid); err != nil {
		return err
	}
	for _, name := range delegationFiles {
		if err := os.Chown(filepath.Join(dir, name), uid, gid); err != nil {
			return err
		}
	}
	return nil
}

// applyDelegatedWrites writes the cappings of a delegated group.
//
// A capping whose file the group does not have is one its controller was not
// delegated for, and it is said so rather than failed on: the other cappings
// hold, and the operator learns which one cannot, and why.
func (c Config) applyDelegatedWrites(writes []unifiedWrite) error {
	var errs error
	dir := filepath.Join(UnifiedPath(), c.Path())
	for _, write := range writes {
		path := filepath.Join(dir, write.file)
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			controller, _, _ := strings.Cut(write.file, ".")
			if c.log != nil {
				c.log.Warnf("pg %s: %s is not capped: the %s controller is not delegated to %s", c.Path(), write.file, controller, c.Delegation.Root)
			}
			continue
		}
		if err := os.WriteFile(path, []byte(write.value), 0644); err != nil {
			errs = errors.Join(errs, fmt.Errorf("write %s: %w", write.file, err))
		}
	}
	return errs
}

// Delete removes the cgroup from the hierarchy the node holds.
func (c Config) Delete() (bool, error) {
	if isUnified() {
		return c.deleteV2()
	}
	return c.deleteV1()
}

func (c Config) deleteV2() (bool, error) {
	control, err := cgroupsv2.LoadManager(UnifiedPath(), c.Path())
	if err != nil {
		// doesn't verify path existence
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
