package omcmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/util/hostname"
)

type (
	CmdDaemonSubAction struct {
		OptsGlobal
		Debug        bool
		NodeSelector string
		Name         string
	}

	apiFuncWithNode func(context.Context, *client.T, string) (*http.Response, error)
)

// Run daemon sub-component action
func (t *CmdDaemonSubAction) Run(fn apiFuncWithNode) error {
	if t.Name == "" {
		return fmt.Errorf("--name must be specified")
	}
	if t.Local {
		t.NodeSelector = hostname.Hostname()
	} else if t.NodeSelector == "" {
		t.NodeSelector = hostname.Hostname()
	}
	c, err := client.New()
	if err != nil {
		return err
	}
	nodenames, err := nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand()
	if errors.Is(err, nodeselector.ErrClusterNodeCacheEmpty) {
		nodenames = []string{hostname.Hostname()}
	} else if err != nil {
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
		running++
		go func(nodename string) {
			errC <- t.doNode(ctx, c, nodename, fn)
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
	if needDoLocal {
		err := t.doNode(ctx, c, hostname.Hostname(), fn)
		errs = errors.Join(errs, err)
	}
	return errs
}

func (t *CmdDaemonSubAction) doNode(ctx context.Context, cli *client.T, nodename string, fn apiFuncWithNode) error {
	resp, err := fn(ctx, cli, nodename)
	if err != nil {
		return fmt.Errorf("action failed on node %s: %w", nodename, err)
	} else if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("action failed on node %s: unexpected status code %d", nodename, resp.StatusCode)
	}
	return nil
}
