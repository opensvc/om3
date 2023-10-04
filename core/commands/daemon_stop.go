package commands

import (
	"context"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/daemon/daemoncmd"
)

type (
	CmdDaemonStop struct {
		OptsGlobal
	}
)

func (t *CmdDaemonStop) Run() error {
	cli, err := client.New(client.WithURL(t.Server))
	if err != nil {
		return err
	}
	ctx := context.Background()
	return daemoncmd.NewContext(ctx, cli).StopFromCmd(ctx)
}
