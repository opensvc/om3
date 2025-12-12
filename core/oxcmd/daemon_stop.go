package oxcmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/clientcontext"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/nodeselector"
	"github.com/opensvc/om3/v3/util/hostname"
)

type (
	CmdDaemonStop struct {
		OptsGlobal
		NodeSelector string
	}
)

func (t *CmdDaemonStop) Run() error {
	if !clientcontext.IsSet() && t.NodeSelector == "" {
		t.NodeSelector = hostname.Hostname()
	}
	if t.NodeSelector == "" {
		return fmt.Errorf("--node must be specified")
	}
	return t.doNodes()
}

func (t *CmdDaemonStop) doNodes() error {
	c, err := client.New()
	if err != nil {
		return err
	}
	nodenames, err := nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand()
	if err != nil {
		return err
	}
	errC := make(chan error)
	ctx := context.Background()
	running := 0
	for _, nodename := range nodenames {
		running++
		go func(nodename string) {
			_, _ = fmt.Printf("stopping daemon on node %s with pid\n", nodename)
			err := commoncmd.PostDaemonStop(ctx, c, nodename)
			errC <- err
		}(nodename)
	}
	var errs error
	for {
		if running == 0 {
			break
		}
		err := <-errC
		errs = errors.Join(errs, err)
		running--
	}
	return errs
}
