package commands

import (
	"fmt"
	"os"
	"time"

	"github.com/opensvc/om3/daemon/daemoncli"
	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/systemd"
)

type (
	CmdDaemonRestart struct {
		OptsGlobal
		Foreground bool
		Debug      bool
	}
)

func (t *CmdDaemonRestart) Run() error {
	if t.Foreground {
		cli, err := newClient(t.Server)
		if err != nil {
			return err
		}
		if err := daemoncli.New(cli).ReStart(); err != nil {
			return err
		}
	} else if os.Getenv("INVOCATION_ID") != "" {
		// called from systemd
		t.nativeRestart()
	} else if systemd.HasSystemd() {
		// systemd is running, delegate to systemd
		return command.New(
			command.WithName("systemctl"),
			command.WithVarArgs("restart", "opensvc-agent"),
		).Run()
	} else {
		t.nativeRestart()
	}
	return nil
}

func (t *CmdDaemonRestart) nativeRestart() {
	args := []string{"daemon", "restart"}
	if t.Log != "" {
		args = append(args, "--log", t.Log)
	}
	if t.Server != "" {
		args = append(args, "--server", t.Server)
	}
	args = append(args, "--foreground")
	cmd := command.New(
		command.WithName(os.Args[0]),
		command.WithArgs(args),
	)
	checker := func() error {
		cli, err := newClient(t.Server)
		if err != nil {
			return err
		}
		time.Sleep(60 * time.Millisecond)
		if err := daemoncli.New(cli).WaitRunning(); err != nil {
			return fmt.Errorf("daemon not running")
		}
		return nil
	}
	daemoncli.LockCmdExit(cmd, checker, "daemon restart")
}
