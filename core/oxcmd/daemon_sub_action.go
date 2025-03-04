package oxcmd

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
		NodeSelector string
		Name         string
	}

	apiFuncWithNode func(context.Context, *client.T, string) (*http.Response, error)
)

// Run executes api function `fn` on multiple nodes concurrently based on the
// `t.NodeSelector`.
func (t *CmdDaemonSubAction) Run(fn apiFuncWithNode) error {
	if t.Name == "" {
		return fmt.Errorf("--name must be specified")
	}
	if t.NodeSelector == "" {
		return fmt.Errorf("--node must be specified")
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
	for _, nodename := range nodenames {
		running++
		go func(nodename string) {
			resp, err := fn(ctx, c, nodename)
			if err != nil {
				err = fmt.Errorf("action failed on node %s: %w", nodename, err)
			} else if resp.StatusCode != http.StatusOK {
				errC <- fmt.Errorf("action failed on node %s: unexpected status code %d", nodename, resp.StatusCode)
			}
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
