package pg

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	cgroups "github.com/containerd/cgroups"
	cgroupsv2 "github.com/containerd/cgroups/v2"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"opensvc.com/opensvc/util/converters"
	"opensvc.com/opensvc/util/sizeconv"
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
)

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

func (c Config) Apply() error {
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

	control, err := cgroupsv2.NewManager("/sys/fs/cgroup/unified", c.ID, cgroupsv2.ToResources(&r))
	if err == nil {
		if err := control.AddProc(uint64(os.Getpid())); err != nil {
			return errors.Wrapf(err, "add pid to cgroup %s", c.ID)
		}
	} else {
		control, err := cgroups.New(cgroups.V1, cgroups.StaticPath(c.ID), &r)
		if err != nil {
			return errors.Wrapf(err, "new cgroup %s", c.ID)
		}
		if err := control.Add(cgroups.Process{Pid: os.Getpid()}); err != nil {
			return errors.Wrapf(err, "add pid to cgroup %s", c.ID)
		}

	}
	return nil
}
