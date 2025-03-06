// Package commoncmd provides utilities and shared functionality to facilitate
// operations related to managing remotes objects, nodes, and logs for omcmd
// and oxcmd.
package commoncmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/xmap"
)

type (
	OptsGlobal struct {
		Color          string
		Output         string
		ObjectSelector string
	}
)

func NodesFromPaths(c *client.T, selector string) ([]string, error) {
	m := make(map[string]any)
	params := api.GetObjectsParams{Path: &selector}
	resp, err := c.GetObjectsWithResponse(context.Background(), &params)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("%s", resp.Status())
	}
	for _, item := range resp.JSON200.Items {
		for node := range item.Data.Instances {
			m[node] = nil
		}
	}
	return xmap.Keys(m), nil
}
