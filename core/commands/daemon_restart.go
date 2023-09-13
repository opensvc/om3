package commands

import (
	"context"

	"github.com/opensvc/om3/daemon/daemoncli"
)

type (
	CmdDaemonRestart struct {
		OptsGlobal
		Debug      bool
		Foreground bool
		CpuProfile string
	}
)

func (t *CmdDaemonRestart) Run() error {
	cli, err := newClient(t.Server)
	if err != nil {
		return err
	}
	ctx := context.Background()
	return daemoncli.NewContext(ctx, cli).RestartFromCmd(ctx, t.Foreground, t.CpuProfile)
}
