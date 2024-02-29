//go:build linux

package osagentservice

import (
	"errors"
	"fmt"
	"os"

	"github.com/containerd/cgroups"
	"github.com/containerd/cgroups/v3/cgroup2"
	"github.com/opensvc/om3/util/capabilities"
	"github.com/opensvc/om3/util/systemd"
)

var (
	agentServiceName = "opensvc-agent.service"
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
	agentSlice := cgroups.Slice("system.slice", agentServiceName)
	cg, err := cgroups.Load(cgroups.Systemd, agentSlice)
	if errors.Is(err, cgroups.ErrCgroupDeleted) {
		p, _ := agentSlice(cgroups.Pids)
		return fmt.Errorf("%s: %w", p, os.ErrNotExist)
	} else if err != nil {
		return err
	}
	return cg.Add(cgroups.Process{Pid: os.Getpid()})
}

func joinV2() error {
	cg, err := cgroup2.LoadSystemd("system.slice", agentServiceName)
	if err != nil {
		return err
	}
	if _, err := cg.GetType(); err != nil {
		return err
	}
	return cg.AddProc(uint64(os.Getpid()))
}
