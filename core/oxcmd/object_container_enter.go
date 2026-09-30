package oxcmd

import (
	"context"
	"fmt"
	"os"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/clientcontext"
	"github.com/opensvc/om3/v3/core/console"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/nodeselector"
	"github.com/opensvc/om3/v3/util/hostname"
)

type (
	CmdObjectContainerEnter struct {
		ObjectSelector string
		RID            string
		NodeSelector   string
	}
)

func (t *CmdObjectContainerEnter) Run(kind string) error {
	if t.NodeSelector == "" {
		if clientcontext.IsSet() {
			return fmt.Errorf("--node must be set")
		}
		t.NodeSelector = hostname.Hostname()
	}
	path, err := naming.ParsePath(t.ObjectSelector)
	if err != nil {
		return err
	}
	c, err := client.New()
	if err != nil {
		return err
	}
	ctx := context.Background()
	nodenames, err := nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand()
	if err != nil {
		return err
	}
	for _, nodename := range nodenames {
		result, err := c.Console(ctx, nodename, path, t.RID, os.Stdin, os.Stdout)
		if err != nil {
			return err
		}
		if result.Reason == console.ReasonError {
			return fmt.Errorf("%s: node %s: %s", path, nodename, result.Text)
		}
	}
	return nil
}
