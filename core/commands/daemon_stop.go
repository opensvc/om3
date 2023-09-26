package commands

import (
	"context"

	"github.com/opensvc/om3/daemon/daemoncmd"
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
	return daemoncmd.NewContext(ctx, cli).StopFromCmd(ctx)
}
