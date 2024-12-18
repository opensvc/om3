package oxcmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/clientcontext"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/util/hostname"
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
			err := t.doNode(ctx, c, nodename)
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

func (t *CmdDaemonStop) doNode(ctx context.Context, cli *client.T, nodename string) error {
	r, err := cli.PostDaemonStopWithResponse(ctx, nodename)
	if err != nil {
		return fmt.Errorf("unexpected post daemon stop failure for %s: %w", nodename, err)
	}
	switch {
	case r.JSON200 != nil:
		_, _ = fmt.Fprintf(os.Stderr, "stopping daemon on remote %s with pid %d\n", nodename, r.JSON200.Pid)
		return nil
	default:
		return fmt.Errorf("unexpected post daemon stop status code for %s: %d", nodename, r.StatusCode())
	}
}
