package oxcmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/clientcontext"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/util/hostname"
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
	if !clientcontext.IsSet() && t.NodeSelector == "" {
		t.NodeSelector = hostname.Hostname()
	}
	if t.NodeSelector == "" {
		return fmt.Errorf("--node must be specified")
	}
	return t.doNodes()
}

func (t *CmdDaemonRestart) doNodes() error {
	c, err := client.New(client.WithURL(t.Server))
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
	needDoLocal := false
	for _, nodename := range nodenames {
		if nodename == hostname.Hostname() {
			needDoLocal = true
			continue
		}
		running += 1
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
		running -= 1
	}
	if needDoLocal {
		_, _ = fmt.Fprintf(os.Stderr, "restarting daemon on localhost\n")
		err := t.doNode(ctx, c, hostname.Hostname())
		errs = errors.Join(errs, err)
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
