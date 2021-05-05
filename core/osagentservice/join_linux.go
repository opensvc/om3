// +build linux

package osagentservice

import (
	"github.com/containerd/cgroups"
	"opensvc.com/opensvc/util/capabilities"
	"opensvc.com/opensvc/util/systemd"
	"os"
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
	cg, err := agentCgroup()
	if err != nil {
		return err
	}
	return cg.Add(cgroups.Process{Pid: os.Getpid()})
}

func agentCgroup() (cgroups.Cgroup, error) {
	agentSlice := cgroups.Slice("system.slice", agentServiceName)
	return cgroups.Load(cgroups.Systemd, agentSlice)
}
