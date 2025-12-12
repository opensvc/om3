package oxcmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/nodeselector"
)

type (
	CmdNodeSSHTrust struct {
		OptsGlobal
		NodeSelector string
	}
)

func (t *CmdNodeSSHTrust) Run() error {
	c, err := client.New(client.WithTimeout(0))
	if err != nil {
		return err
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
		resp, err := c.PutNodeSSHTrustWithResponse(ctx, node)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("%s: %w", node, err))
			continue
		}
		switch resp.StatusCode() {
		case 204:
		case 401:
			errs = errors.Join(errs, fmt.Errorf("%s: %s: %s", node, resp.JSON401.Title, resp.JSON401.Detail))
		case 403:
			errs = errors.Join(errs, fmt.Errorf("%s: %s: %s", node, resp.JSON403.Title, resp.JSON403.Detail))
		case 500:
			errs = errors.Join(errs, fmt.Errorf("%s: %s: %s", node, resp.JSON500.Title, resp.JSON500.Detail))
		default:
			errs = errors.Join(errs, fmt.Errorf("%s: unexpected status: %s", node, resp.Status()))
		}
	}
	return errs
}
