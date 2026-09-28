//go:build linux

package osagentservice

import (
	"errors"
	"fmt"
	"os"

	"github.com/containerd/cgroups/v3"
	"github.com/containerd/cgroups/v3/cgroup1"
	"github.com/containerd/cgroups/v3/cgroup2"

	"github.com/opensvc/om3/v3/daemon/daemonsys"
	"github.com/opensvc/om3/v3/util/capabilities"
	"github.com/opensvc/om3/v3/util/systemd"
)

// Join add current process to opensvc systemd agent service when
// node has systemd capability
func Join() error {
	if !capabilities.Has(systemd.NodeCapability) {
		return nil
	}

	if cgroups.Mode() == cgroups.Unified {
		return joinV2()
	} else {
		return joinV1()
	}
}

func joinV1() error {
	agentSlice := cgroup1.Slice("system.slice", daemonsys.UnitName)
	cg, err := cgroup1.Load(agentSlice, cgroup1.WithHierarchy(cgroup1.Systemd))
	if errors.Is(err, cgroup1.ErrCgroupDeleted) {
		p, _ := agentSlice(cgroup1.Pids)
		return fmt.Errorf("%s: %w", p, os.ErrNotExist)
	} else if err != nil {
		return err
	}
	return cg.Add(cgroup1.Process{Pid: os.Getpid()})
}

func joinV2() error {
	cg, err := cgroup2.LoadSystemd("system.slice", daemonsys.UnitName)
	if err != nil {
		return err
	}
	if _, err := cg.GetType(); err != nil {
		return err
	}
	return cg.AddProc(uint64(os.Getpid()))
}
