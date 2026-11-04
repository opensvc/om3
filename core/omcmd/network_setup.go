package omcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/network"
	"github.com/opensvc/om3/v3/core/nodeselector"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/util/hostname"
)

type (
	CmdNetworkSetup struct {
		OptsGlobal
		Names        []string
		NodeSelector string
	}
)

// Run asks the node monitors to set up the networks, which run one setup at
// a time. With the daemon of this node down, there is no setup running to
// collide with: the networks of this node are set up here.
func (t *CmdNetworkSetup) Run() error {
	if t.NodeSelector == "" {
		t.NodeSelector = hostname.Hostname()
	}
	c, err := client.New(client.WithTimeout(commoncmd.NetworkSetupClientTimeout))
	if err != nil {
		return err
	}
	nodenames, err := nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand()
	if err != nil {
		if client.IsDaemonDown(err) && t.NodeSelector == hostname.Hostname() {
			return t.doLocal()
		}
		return err
	}
	err = commoncmd.NetworkSetup(context.Background(), c, nodenames, t.Names, t.Output)
	if client.IsDaemonDown(err) && t.NodeSelector == hostname.Hostname() {
		return t.doLocal()
	}
	return err
}

func (t *CmdNetworkSetup) doLocal() error {
	fmt.Println("the daemon is not running: set up the networks here")
	n, err := object.NewNode()
	if err != nil {
		return err
	}
	return network.Setup(n, t.Names...)
}
