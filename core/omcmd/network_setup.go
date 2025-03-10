package omcmd

import (
	"fmt"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/network"
	"github.com/opensvc/om3/core/object"
)

type (
	CmdNetworkSetup struct {
		OptsGlobal
	}
)

func (t *CmdNetworkSetup) Run() error {
	if t.Local {
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
	if c, err = client.New(); err != nil {
		return err
	}
	return fmt.Errorf("todo %v", c)
}
