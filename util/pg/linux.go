//go:build linux

package pg

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

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
func (c Config) ApplyProc(pid int) (created bool, errs error) {
	if c.ID == "" {
		errs = fmt.Errorf("the pg config application requires a non empty pg id")
		return
	}
	r := specs.LinuxResources{
		CPU:     &specs.LinuxCPU{},
		Memory:  &specs.LinuxMemory{},
		BlockIO: &specs.LinuxBlockIO{},
	}
	writes := make([]unifiedWrite, 0)
	var hasReset bool
	reset := func(kw string) {
		hasReset = true
		writes = append(writes, unifiedResets[kw])
	}

	if c.CPUShares == DefaultValue {
		reset("pg_cpu_shares")
	} else if n, err := sizeconv.FromSize(c.CPUShares); err == nil {
		shares := uint64(n)
		r.CPU.Shares = &shares
	}
	if c.CPUs == DefaultValue {
		reset("pg_cpus")
	} else if c.CPUs != "" {
		r.CPU.Cpus = c.CPUs
	}
	if c.Mems == DefaultValue {
		reset("pg_mems")
	} else if c.Mems != "" {
		r.CPU.Mems = c.Mems
	}
	if c.CPUQuota == DefaultValue {
		reset("pg_cpu_quota")
	} else if c.CPUQuota != "" {
		period := uint64(100000)
		if quota, err := CPUQuota(c.CPUQuota).Convert(period); err == nil {
			r.CPU.Period = &period
			r.CPU.Quota = &quota
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
			if c.VMemLimit != "" {
				if n, err := sizeconv.FromSize(c.VMemLimit); err != nil {
					errs = errors.Join(errs, fmt.Errorf("pg_vmem_limit: %w", err))
				} else {
					n -= *r.Memory.Limit
					r.Memory.Swap = &n
				}
			}
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
	// The block io weight is written where the unified hierarchy keeps it,
	// io.weight, and is left out of the resources handed to the manager: the
	// manager writes io.bfq.weight, which a kernel not running the bfq
	// scheduler does not have, and failing to write it fails the whole apply,
	// so a weight took every other capping of the object down with it. The v2
	// agent writes io.weight too, and the value as it is written, where the
	// manager rescales a weight of 500 to 4950.
	//
	// blockIOWeight is kept for the v1 hierarchy, where blkio.weight is the
	// file and the manager is right.
	var blockIOWeight *uint16
	if c.BlockIOWeight == DefaultValue {
		reset("pg_blkio_weight")
	} else if c.BlockIOWeight != "" {
		if n, err := strconv.ParseUint(c.BlockIOWeight, 10, 16); err != nil {
			errs = errors.Join(errs, fmt.Errorf("pg_blkio_weight: %w", err))
		} else {
			weight := uint16(n)
			blockIOWeight = &weight
			writes = append(writes, unifiedWrite{"io.weight", c.BlockIOWeight})
		}
	}

	control, err := cgroupsv2.NewManager(UnifiedPath(), c.ID, cgroupsv2.ToResources(&r))
	if err == nil {
		if len(writes) > 0 {
			errs = errors.Join(errs, applyUnifiedWrites(c.ID, writes))
		}
		if pid == 0 {
			// pass
		} else if err := control.AddProc(uint64(pid)); err != nil {
			errs = errors.Join(errs, fmt.Errorf("add pid to pg %s: %w", c.ID, err))
		}
	} else {
		if hasReset {
			// The v1 hierarchy names these files otherwise and holds other
			// values in them, and none of that was read from a v1 node.
			errs = errors.Join(errs, fmt.Errorf("a pg keyword set to %q needs the unified cgroup hierarchy", DefaultValue))
		} else if blockIOWeight != nil {
			// blkio.weight is the v1 file, and the manager writes it.
			r.BlockIO.Weight = blockIOWeight
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
	}
	return
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
	control, err := cgroupsv2.LoadManager(UnifiedPath(), c.ID)
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
