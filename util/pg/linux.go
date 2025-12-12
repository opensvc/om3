//go:build linux

package pg

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/containerd/cgroups"
	cgroupsv2 "github.com/containerd/cgroups/v2"
	"github.com/opencontainers/runtime-spec/specs-go"

	"github.com/opensvc/om3/v3/util/converters"
	"github.com/opensvc/om3/v3/util/sizeconv"
)

// ApplyProc creates the cgroup, set caps, and add the specified process
func (c Config) ApplyProc(pid int) error {
	var errs error
	if c.ID == "" {
		return fmt.Errorf("the pg config application requires a non empty pg id")
	}
	r := specs.LinuxResources{
		CPU:     &specs.LinuxCPU{},
		Memory:  &specs.LinuxMemory{},
		BlockIO: &specs.LinuxBlockIO{},
	}
	if n, err := sizeconv.FromSize(c.CPUShares); err == nil {
		shares := uint64(n)
		r.CPU.Shares = &shares
	}
	if c.CPUs != "" {
		r.CPU.Cpus = c.CPUs
	}
	if c.Mems != "" {
		r.CPU.Mems = c.Mems
	}
	if c.CPUQuota != "" {
		period := uint64(100000)
		if quota, err := CPUQuota(c.CPUQuota).Convert(period); err == nil {
			r.CPU.Period = &period
			r.CPU.Quota = &quota
		}
	}
	if c.MemLimit != "" {
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
		if n, err := converters.Lookup("bool").Convert(c.MemOOMControl); err != nil {
			errs = errors.Join(errs, fmt.Errorf("pg_mem_oom_control: %w", err))
		} else {
			disable := n.(bool)
			r.Memory.DisableOOMKiller = &disable
		}
	}
	if c.BlockIOWeight != "" {
		if n, err := strconv.ParseUint(c.BlockIOWeight, 10, 16); err != nil {
			errs = errors.Join(errs, fmt.Errorf("pg_blkio_weight: %w", err))
		} else {
			weight := uint16(n)
			r.BlockIO.Weight = &weight
		}
	}

	control, err := cgroupsv2.NewManager(UnifiedPath(), c.ID, cgroupsv2.ToResources(&r))
	if err == nil {
		if pid == 0 {
			// pass
		} else if err := control.AddProc(uint64(pid)); err != nil {
			errs = errors.Join(errs, fmt.Errorf("add pid to pg %s: %w", c.ID, err))
		}
	} else {
		control, err := cgroups.New(cgroups.V1, cgroups.StaticPath(c.ID), &r)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("new pg %s: %w", c.ID, err))
		} else if pid == 0 {
			// pass
		} else if err := control.Add(cgroups.Process{Pid: pid}); err != nil {
			errs = errors.Join(errs, fmt.Errorf("add pid to pg %s: %w", c.ID, err))
		}
	}
	return errs
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
