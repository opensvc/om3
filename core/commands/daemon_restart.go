package commands

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/daemon/daemoncmd"
)

type (
	CmdDaemonRestart struct {
		OptsGlobal
		Debug bool
	}
)

// Run functions restart daemon.
//
// The daemon restart is asynchronous when node selector is used
func (t *CmdDaemonRestart) Run() error {
	if t.NodeSelector == "" {
		return t.restartLocal()
	} else {
		nodes, err := nodeselector.New(t.NodeSelector).Expand()
		if err != nil {
			return fmt.Errorf("can't retrieve nodes: %w", err)
		} else if len(nodes) == 0 {
			return fmt.Errorf("empty nodes")
		}
		return t.restartRemotes(nodes)
	}
}

func (t *CmdDaemonRestart) restartLocal() error {
	_, _ = fmt.Fprintf(os.Stderr, "restarting daemon on localhost\n")
	cli, err := client.New()
	if err != nil {
		return err
	}
	ctx := context.Background()
	return daemoncmd.NewContext(ctx, cli).RestartFromCmd(ctx)
}

func (t *CmdDaemonRestart) restartRemotes(nodes []string) error {
	var errs error
	for _, s := range nodes {
		if err := t.restartRemote(s); err != nil {
			errs = errors.Join(errs, fmt.Errorf("restart daemon failed on %s: %w", s, err))
		}
	}
	return errs
}

func (t *CmdDaemonRestart) restartRemote(s string) error {
	_, _ = fmt.Fprintf(os.Stderr, "restarting daemon on remote %s\n", s)
	cli, err := newClient(s)
	if err != nil {
		return err
	}
	ctx := context.Background()
	r, err := cli.PostDaemonRestart(ctx)
	if err != nil {
		return fmt.Errorf("unexpected post daemon restart failure for %s: %w", s, err)
	}
	switch r.StatusCode {
	case http.StatusOK:
		return nil
	default:
		return fmt.Errorf("unexpected post daemon restart status code for %s: %d", s, r.StatusCode)
	}
}
