package pg

import (
	"fmt"
	"os"
	"strconv"

	"github.com/containerd/cgroups"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"opensvc.com/opensvc/util/converters"
	"opensvc.com/opensvc/util/sizeconv"
)

type Config struct {
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

func (c Config) Apply() error {
	if c.ID == "" {
		return fmt.Errorf("Config Path is mandatory")
	}
	control, err := cgroups.New(cgroups.V1, cgroups.StaticPath(c.ID), &specs.LinuxResources{})
	if err != nil {
		return errors.Wrapf(err, "new cgroup %s", c.ID)
	}
	if err := control.Add(cgroups.Process{Pid: os.Getpid()}); err != nil {
		return errors.Wrapf(err, "add pid to cgroup %s", c.ID)
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
	if n, err := strconv.ParseInt(c.CpuQuota, 10, 64); err == nil {
		r.CPU.Quota = &n
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

	if err := control.Update(&r); err != nil {
		return errors.Wrapf(err, "update cgroup %s: %+v", c.ID, r)
	}
	return nil
}
