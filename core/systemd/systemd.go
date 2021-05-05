package systemd

import (
	"io/ioutil"
	"os"

	"github.com/containerd/cgroups"
	"opensvc.com/opensvc/config"
)

var (
	procOneComm      = "/proc/1/comm"
	agentServiceName = "opensvc-agent.service"
)

func HasSystemd() bool {
	var (
		b   []byte
		err error
	)
	if b, err = ioutil.ReadFile(procOneComm); err != nil {
		return false
	}
	return string(b) == "systemd\n"
}

func JoinAgentCgroup() error {
	if !HasSystemd() {
		return nil
	}
	if config.HasDaemonOrigin() {
		return nil
	}
	control, err := cgroups.Load(cgroups.Systemd, cgroups.Slice("system.slice", agentServiceName))
	if err != nil {
		return err
	}
	if err := control.Add(cgroups.Process{Pid: os.Getpid()}); err != nil {
		return err
	}
	return nil
}
