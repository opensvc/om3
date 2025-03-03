package omcmd

import (
	"context"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/daemon/daemoncmd"
)

type (
	CmdDaemonRun struct {
		OptsGlobal
		CPUProfile string
	}
)

func (t *CmdDaemonRun) Run() error {
	cli, err := client.New()
	if err != nil {
		return err
	}
	ctx := context.Background()
	return daemoncmd.NewContext(ctx, cli).StartFromCmd(ctx, true, t.CPUProfile)
}
