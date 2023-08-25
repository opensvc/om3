package commands

import (
	"os"

	"github.com/opensvc/om3/daemon/daemoncli"
	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/systemd"
)

type (
	CmdDaemonStop struct {
		OptsGlobal
	}
)

func (t *CmdDaemonStop) Run() error {
	if os.Getenv("INVOCATION_ID") != "" || !systemd.HasSystemd() {
		// called from systemd, or systemd is not running
		return t.nativeStop()
	}
	// systemd is running, ask systemd to stop opensvc-agent unit
	return command.New(
		command.WithName("systemctl"),
		command.WithVarArgs("stop", "opensvc-agent"),
	).Run()
}

func (t *CmdDaemonStop) nativeStop() error {
	cli, err := newClient(t.Server)
	if err != nil {
		return err
	}
	daemoncli.LockFuncExit("daemon stop", daemoncli.New(cli).Stop)
	return nil
}
