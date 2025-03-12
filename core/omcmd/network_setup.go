package omcmd

import (
	"github.com/opensvc/om3/core/network"
	"github.com/opensvc/om3/core/object"
)

type (
	CmdNetworkSetup struct {
		OptsGlobal
	}
)

func (t *CmdNetworkSetup) Run() error {
	return t.doLocal()
}

func (t *CmdNetworkSetup) doLocal() error {
	n, err := object.NewNode()
	if err != nil {
		return err
	}
	return network.Setup(n)
}
