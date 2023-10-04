package commands

import (
	"context"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/daemon/daemoncmd"
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
	cli, err := client.New(client.WithURL(t.Server))
	if err != nil {
		return err
	}
	ctx := context.Background()
	return daemoncmd.NewContext(ctx, cli).StartFromCmd(ctx, t.Foreground, t.CpuProfile)
}
