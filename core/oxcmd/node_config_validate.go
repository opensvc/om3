package oxcmd

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/nodeselector"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	CmdNodeConfigValidate struct {
		OptsGlobal
		NodeSelector string
	}
)

// Run validates the configuration of the selected nodes, each on the node.
func (t *CmdNodeConfigValidate) Run() error {
	c, err := client.New()
	if err != nil {
		return err
	}
	nodes, err := nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand()
	if err != nil {
		return err
	}
	if len(nodes) == 0 {
		return fmt.Errorf("no node matching %s", t.NodeSelector)
	}
	var (
		mu   sync.Mutex
		wg   sync.WaitGroup
		errs error
	)
	ctx := context.Background()
	alerts := api.ConfigAlertList{Kind: "ConfigAlertList", Items: make(api.ConfigAlertItems, 0)}
	for _, node := range nodes {
		wg.Add(1)
		go func(node string) {
			defer wg.Done()
			items, err := t.validate(ctx, c, node)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = errors.Join(errs, err)
				return
			}
			alerts.Items = append(alerts.Items, items...)
		}(node)
	}
	wg.Wait()
	slices.SortStableFunc(alerts.Items, func(a, b api.ConfigAlert) int {
		if a.Node < b.Node {
			return -1
		} else if a.Node > b.Node {
			return 1
		}
		return 0
	})
	errs = errors.Join(errs, output.Renderer{
		DefaultOutput: "tab=LEVEL:level,NODE:node,DRIVER:driver,KEY:key,KIND:kind,COMMENT:comment",
		Output:        t.Output,
		Sort:          t.Sort,
		Color:         t.Color,
		Data:          alerts,
		Colorize:      rawconfig.Colorize,
	}.Print())
	return errors.Join(errs, configAlertErrors(alerts.Items))
}

func (t *CmdNodeConfigValidate) validate(ctx context.Context, c *client.T, node string) (api.ConfigAlertItems, error) {
	resp, err := c.GetNodeConfigValidateWithResponse(ctx, node)
	if err != nil {
		return nil, fmt.Errorf("node %s: %w", node, err)
	}
	switch {
	case resp.JSON200 != nil:
		return resp.JSON200.Items, nil
	case resp.JSON400 != nil:
		return nil, fmt.Errorf("node %s: %s", node, resp.JSON400.Detail)
	case resp.JSON401 != nil:
		return nil, fmt.Errorf("node %s: %s", node, resp.JSON401.Detail)
	case resp.JSON403 != nil:
		return nil, fmt.Errorf("node %s: %s", node, resp.JSON403.Detail)
	case resp.JSON404 != nil:
		return nil, fmt.Errorf("node %s: %s", node, resp.JSON404.Detail)
	case resp.JSON500 != nil:
		return nil, fmt.Errorf("node %s: %s", node, resp.JSON500.Detail)
	default:
		return nil, fmt.Errorf("node %s: unexpected response: %s", node, resp.Status())
	}
}
