package oxcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/clientcontext"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/nodeselector"
	"github.com/opensvc/om3/v3/util/hostname"
)

type (
	CmdNetworkSetup struct {
		OptsGlobal
		Names        []string
		NodeSelector string
	}
)

func (t *CmdNetworkSetup) Run() error {
	if !clientcontext.IsSet() && t.NodeSelector == "" {
		t.NodeSelector = hostname.Hostname()
	}
	if t.NodeSelector == "" {
		return fmt.Errorf("--node must be specified")
	}
	c, err := client.New(client.WithTimeout(commoncmd.NetworkSetupClientTimeout))
	if err != nil {
		return err
	}
	nodenames, err := nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand()
	if err != nil {
		return err
	}
	return commoncmd.NetworkSetup(context.Background(), c, nodenames, t.Names, t.Output)
}
