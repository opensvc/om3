package oxcmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/nodeselector"
)

type (
	CmdDaemonRestart struct {
		OptsGlobal
		NodeSelector string
	}
)

// Run functions restart daemon.
//
// The daemon restart is asynchronous when node selector is used
func (t *CmdDaemonRestart) Run() error {
	if t.NodeSelector == "" {
		return fmt.Errorf("--node must be specified")
	}
	return t.doNodes()
}

func (t *CmdDaemonRestart) doNodes() error {
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
			_, _ = fmt.Fprintf(os.Stderr, "restarting daemon on remote %s\n", nodename)
			err := t.doNode(ctx, c, nodename)
			errC <- err
		}(nodename)
	}
	var (
		errs error
	)
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

func (t *CmdDaemonRestart) doNode(ctx context.Context, cli *client.T, nodename string) error {
	r, err := cli.PostDaemonRestart(ctx, nodename)
	if err != nil {
		return fmt.Errorf("unexpected post daemon restart failure for %s: %w", nodename, err)
	}
	switch r.StatusCode {
	case http.StatusOK:
		return nil
	default:
		return fmt.Errorf("unexpected post daemon restart status code for %s: %d", nodename, r.StatusCode)
	}
}
