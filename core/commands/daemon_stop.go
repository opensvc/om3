package commands

import (
	"context"

	"github.com/opensvc/om3/daemon/daemoncli"
)

type (
	CmdDaemonStop struct {
		OptsGlobal
	}
)

func (t *CmdDaemonStop) Run() error {
	cli, err := newClient(t.Server)
	if err != nil {
		return err
	}
	ctx := context.Background()
	return daemoncli.NewContext(ctx, cli).StopFromCmd(ctx)
}
