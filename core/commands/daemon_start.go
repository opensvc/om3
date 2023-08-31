package commands

import (
	"context"

	"github.com/opensvc/om3/daemon/daemoncli"
)

type (
	CmdDaemonStart struct {
		OptsGlobal
		Debug      bool
		Foreground bool
		CpuProfile string
	}
)

func (t *CmdDaemonStart) cmdArgs() []string {
	var args []string
	if t.Log != "" {
		args = append(args, "--log", t.Log)
	}
	if t.Server != "" {
		args = append(args, "--server", t.Server)
	}
	return args
}

func (t *CmdDaemonStart) Run() error {
	cli, err := newClient(t.Server)
	if err != nil {
		return err
	}
	ctx := context.Background()
	return daemoncli.NewContext(ctx, cli).StartFromCmd(ctx, t.Foreground, t.CpuProfile)
}
