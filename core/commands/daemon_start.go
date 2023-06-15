package commands

import (
	"fmt"
	"os"
	"runtime/pprof"
	"time"

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
				return fmt.Errorf("Create CPU profile: %w", err)
			}
			defer f.Close() // error handling omitted for example
			if err := pprof.StartCPUProfile(f); err != nil {
				return fmt.Errorf("Start CPU profile: %w", err)
			}
			defer pprof.StopCPUProfile()
		}
		if err := daemoncli.New(cli).Start(); err != nil {
			return fmt.Errorf("Start daemon cli: %w", err)
		}
	} else {
		args := []string{"daemon", "start", "--foreground"}
		if t.Log != "" {
			args = append(args, "--log", t.Log)
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
				return fmt.Errorf("Daemon not running: %w", err)
			}
			return nil
		}
		daemoncli.LockCmdExit(cmd, checker, "daemon start")
	}
	return nil
}
