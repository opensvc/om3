package commands

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/daemon/daemoncli"
	"github.com/opensvc/om3/util/hostname"
)

type (
	CmdDaemonRestart struct {
		OptsGlobal
		Debug      bool
		Foreground bool
	}
)

// Run functions restart daemon.
//
// The daemon restart is asynchronous when node selector is used,
// or synchronous when --local is used or node selector is empty
func (t *CmdDaemonRestart) Run() error {
	var (
		errs      error
		localhost = hostname.Hostname()
		nodes     []string
	)

	if t.Local || t.NodeSelector == "" {
		if err := t.restartLocal(); err != nil {
			return fmt.Errorf("can't restart daemon on %s: %w", localhost, err)
		}
		return nil
	}
	if t.Foreground {
		return fmt.Errorf("can't restart daemon on remote with foreground option")
	}
	if nodes, errs = nodeselector.New(t.NodeSelector).Expand(); errs != nil {
		return fmt.Errorf("daemon restart unable retrieve target nodes: %w", errs)
	} else if len(nodes) == 0 {
		return fmt.Errorf("daemon restart failed: empty nodes")
	}
	for _, s := range nodes {
		if err := t.restartRemote(s); err != nil {
			errs = errors.Join(errs, fmt.Errorf("can't restart daemon on %s: %w", s, err))
		}
	}
	return errs
}

func (t *CmdDaemonRestart) restartLocal() error {
	_, _ = fmt.Fprintf(os.Stderr, "restarting daemon on localhost\n")
	cli, err := client.New()
	if err != nil {
		return err
	}
	ctx := context.Background()
	return daemoncli.NewContext(ctx, cli).RestartFromCmd(ctx, t.Foreground)
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
