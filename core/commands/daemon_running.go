package commands

import (
	"os"

	"opensvc.com/opensvc/daemon/daemoncli"
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
	dCli := daemoncli.New(cli)
	dCli.SetNode(t.NodeSelector)
	if !dCli.Running() {
		os.Exit(1)
	}
	return nil
}
