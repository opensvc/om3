//go:build linux

package osagentservice

import (
	"os"

	"github.com/containerd/cgroups"
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
