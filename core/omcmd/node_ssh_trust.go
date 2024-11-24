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
	CmdNodeSSHTrust struct {
		OptsGlobal
		NodeSelector string
	}
)

func (t *CmdNodeSSHTrust) Run() error {
	c, err := client.New(client.WithURL(t.Server), client.WithTimeout(0))
	if err != nil {
		return err
	}
	if t.NodeSelector == "" {
		t.NodeSelector = hostname.Hostname()
	}
	nodes, err := nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand()
	if err != nil {
		return err
	}
	if len(nodes) == 0 {
		return fmt.Errorf("no nodes found")
	}
	ctx := context.Background()
	var errs error
	for _, node := range nodes {
		resp, err := c.PutNodeSSHTrust(ctx, node)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("%s: %w", node, err))
			continue
		}
		if resp.StatusCode != http.StatusNoContent {
			errs = errors.Join(errs, fmt.Errorf("%s: unexpected status code: %s", node, resp.Status))
		}
	}
	return errs
}
