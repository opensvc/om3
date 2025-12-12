package omcmd

import (
	"os"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/daemoncmd"
)

type (
	CmdDaemonRunning struct {
		OptsGlobal
		NodeSelector string
	}
)

func (t *CmdDaemonRunning) Run() error {
	cli, err := client.New()
	if err != nil {
		return err
	}
	dCli := daemoncmd.New(cli)
	dCli.SetNode(t.NodeSelector)
	if isRunning, err := dCli.IsRunning(); err != nil {
		return err
	} else if !isRunning {
		os.Exit(1)
	}
	return nil
}
