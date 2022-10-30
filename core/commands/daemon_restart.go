package commands

import (
	"os"
	"time"

	"github.com/pkg/errors"
	"opensvc.com/opensvc/daemon/daemoncli"
	"opensvc.com/opensvc/util/command"
)

type (
	CmdDaemonRestart struct {
		OptsGlobal
		Foreground bool
		Debug      bool
	}
)

func (t *CmdDaemonRestart) Run() error {
	cli, err := newClient(t.Server)
	if err != nil {
		return err
	}
	if t.Foreground {
		if err := daemoncli.New(cli).ReStart(); err != nil {
			return err
		}
	} else {
		args := []string{"daemon", "restart"}
		if t.Debug {
			args = append(args, "--debug")
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
			time.Sleep(60 * time.Millisecond)
			if err := daemoncli.New(cli).WaitRunning(); err != nil {
				return errors.New("daemon not running")
			}
			return nil
		}
		daemoncli.LockCmdExit(cmd, checker, "daemon restart")
	}
	return nil
}
