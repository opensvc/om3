package commands

import (
	"fmt"
	"os"
	"runtime/pprof"
	"time"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/daemon/daemoncli"
	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/systemd"
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
		return t.runForeground(cli)
	} else if os.Getenv("INVOCATION_ID") != "" {
		// called from systemd
		cmd := t.startNativeCmd()
		return t.runBackground(cli, cmd)
	} else {
		// not called from systemd
		var cmd *command.T
		if systemd.HasSystemd() {
			// systemd is running, delegate to systemd
			cmd = t.startUnitCmd()
			return cmd.Run()
		} else {
			// systemd is not running, start background
			cmd = t.startNativeCmd()
			return t.runBackground(cli, cmd)
		}
	}
}

func (t *CmdDaemonStart) runBackground(cli *client.T, cmd *command.T) error {
	checker := func() error {
		time.Sleep(60 * time.Millisecond)
		if err := daemoncli.New(cli).WaitRunning(); err != nil {
			return fmt.Errorf("daemon not running: %w", err)
		}
		return nil
	}
	daemoncli.LockCmdExit(cmd, checker, "daemon start")
	return nil
}

func (t *CmdDaemonStart) runForeground(cli *client.T) error {
	if t.CpuProfile != "" {
		f, err := os.Create(t.CpuProfile)
		if err != nil {
			return fmt.Errorf("create CPU profile: %w", err)
		}
		defer f.Close() // error handling omitted for example
		if err := pprof.StartCPUProfile(f); err != nil {
			return fmt.Errorf("start CPU profile: %w", err)
		}
		defer pprof.StopCPUProfile()
	}
	if err := daemoncli.New(cli).Start(); err != nil {
		return fmt.Errorf("start daemon cli: %w", err)
	}
	return nil
}

func (t *CmdDaemonStart) startUnitCmd() *command.T {
	return command.New(
		command.WithName("systemctl"),
		command.WithVarArgs("start", "opensvc-agent"),
	)
}

func (t *CmdDaemonStart) startNativeCmd() *command.T {
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
	return cmd
}
