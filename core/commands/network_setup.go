package commands

import (
	"github.com/pkg/errors"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/clientcontext"
	"opensvc.com/opensvc/core/network"
	"opensvc.com/opensvc/core/object"
)

type (
	CmdNetworkSetup struct {
		OptsGlobal
	}
)

func (t *CmdNetworkSetup) Run() error {
	if t.Local || !clientcontext.IsSet() {
		return t.doLocal()
	} else {
		return t.doDaemon()
	}
}

func (t *CmdNetworkSetup) doLocal() error {
	n, err := object.NewNode()
	if err != nil {
		return err
	}
	return network.Setup(n)
}

func (t *CmdNetworkSetup) doDaemon() error {
	var (
		c   *client.T
		err error
	)
	if c, err = client.New(client.WithURL(t.Server)); err != nil {
		return err
	}
	return errors.Errorf("TODO %v", c)
}
