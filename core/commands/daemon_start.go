package commands

import (
	"os"
	"runtime/pprof"
	"time"

	"github.com/pkg/errors"
	"github.com/opensvc/om3/daemon/daemoncli"
	"github.com/opensvc/om3/util/command"
)

type (
	CmdDaemonStart struct {
		OptsGlobal
		Debug      bool
		Foreground bool
		CpuProfile string
	}
)

func (t *CmdDaemonStart) Run() error {
	cli, err := newClient(t.Server)
	if err != nil {
		return err
	}
	if t.Foreground {
		if t.CpuProfile != "" {
			f, err := os.Create(t.CpuProfile)
			if err != nil {
				return errors.Wrap(err, "create CPU profile")
			}
			defer f.Close() // error handling omitted for example
			if err := pprof.StartCPUProfile(f); err != nil {
				return errors.Wrap(err, "start CPU profile")
			}
			defer pprof.StopCPUProfile()
		}
		if err := daemoncli.New(cli).Start(); err != nil {
			return errors.Wrap(err, "start daemon cli")
		}
	} else {
		args := []string{"daemon", "start", "--foreground"}
		if t.Debug {
			args = append(args, "--debug")
		}
		if t.Server != "" {
			args = append(args, "--server", t.Server)
		}
		cmd := command.New(
			command.WithName(os.Args[0]),
			command.WithArgs(args),
		)
		checker := func() error {
			time.Sleep(60 * time.Millisecond)
			if err := daemoncli.New(cli).WaitRunning(); err != nil {
				return errors.Wrap(err, "daemon not running")
			}
			return nil
		}
		daemoncli.LockCmdExit(cmd, checker, "daemon start")
	}
	return nil
}
