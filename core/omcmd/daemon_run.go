package omcmd

import (
	"context"
	"errors"
	"fmt"

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
	cmd := daemoncmd.New(cli)
	if err := cmd.LoadManager(ctx); err != nil {
		return err
	}
	if cmd.Run(ctx, t.CPUProfile); errors.Is(err, daemoncmd.ErrAlreadyRunning) {
		fmt.Println(err)
		return nil
	} else if err != nil {
		return err
	} else {
		return nil
	}
}
