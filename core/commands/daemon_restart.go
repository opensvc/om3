package commands

import (
	"os"
	"time"

	"github.com/opensvc/om3/daemon/daemoncli"
	"github.com/opensvc/om3/util/command"
	"github.com/pkg/errors"
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
