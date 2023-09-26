package commands

import (
	"os"

	"github.com/opensvc/om3/daemon/daemoncmd"
)

type (
	CmdDaemonRunning struct {
		OptsGlobal
	}
)

func (t *CmdDaemonRunning) Run() error {
	cli, err := newClient(t.Server)
	if err != nil {
		return err
	}
	dCli := daemoncmd.New(cli)
	dCli.SetNode(t.NodeSelector)
	if !dCli.Running() {
		os.Exit(1)
	}
	return nil
}
