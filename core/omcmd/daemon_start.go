package omcmd

import (
	"context"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/daemon/daemoncmd"
)

type (
	CmdDaemonStart struct {
		OptsGlobal
		Foreground bool
		CPUProfile string
	}
)

func (t *CmdDaemonStart) cmdArgs() []string {
	var args []string
	if t.Quiet {
		args = append(args, "--quiet")
	}
	if t.Debug {
		args = append(args, "--debug")
	}
	return args
}

func (t *CmdDaemonStart) Run() error {
	cli, err := client.New()
	if err != nil {
		return err
	}
	ctx := context.Background()
	return daemoncmd.NewContext(ctx, cli).StartFromCmd(ctx, t.Foreground, t.CPUProfile)
}
