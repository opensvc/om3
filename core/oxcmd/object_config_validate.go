package oxcmd

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/nodeselector"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	CmdObjectConfigValidate struct {
		OptsGlobal
		NodeSelector string
	}
)

// Run validates the configuration of the selected objects on the selected
// nodes of each, as om validate config does on the node it runs on: the
// checks the drivers make loading the resources are the node's.
func (t *CmdObjectConfigValidate) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	c, err := client.New()
	if err != nil {
		return err
	}
	ctx := context.Background()
	resp, err := c.GetObjectsWithResponse(ctx, &api.GetObjectsParams{Path: &mergedSelector})
	if err != nil {
		return err
	} else if resp.JSON200 == nil {
		return fmt.Errorf("get objects: %s", resp.Status())
	}
	nodes, err := nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand()
	if err != nil {
		return err
	}
	type target struct {
		path naming.Path
		node string
	}
	targets := make([]target, 0)
	for _, item := range resp.JSON200.Items {
		p, err := naming.ParsePath(item.Meta.Object)
		if err != nil {
			return err
		}
		core, err := item.Data.AsObjectCore()
		if err != nil {
			return err
		}
		for _, node := range nodes {
			// An object scoped to no node, as a sec, is on every node.
			if len(core.Scope) == 0 || slices.Contains(core.Scope, node) {
				targets = append(targets, target{path: p, node: node})
			}
		}
	}
	if len(targets) == 0 {
		if !t.IgnoreNotFound {
			return fmt.Errorf("no instance of %s on %s", mergedSelector, t.NodeSelector)
		}
		return nil
	}
	var (
		mu   sync.Mutex
		wg   sync.WaitGroup
		errs error
	)
	alerts := api.ConfigAlertList{Kind: "ConfigAlertList", Items: make(api.ConfigAlertItems, 0)}
	for _, e := range targets {
		wg.Add(1)
		go func(p naming.Path, node string) {
			defer wg.Done()
			items, err := t.validate(ctx, c, p, node)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = errors.Join(errs, err)
				return
			}
			alerts.Items = append(alerts.Items, items...)
		}(e.path, e.node)
	}
	wg.Wait()
	slices.SortStableFunc(alerts.Items, func(a, b api.ConfigAlert) int {
		if a.Path != b.Path {
			if a.Path < b.Path {
				return -1
			}
			return 1
		}
		if a.Node < b.Node {
			return -1
		} else if a.Node > b.Node {
			return 1
		}
		return 0
	})
	errs = errors.Join(errs, output.Renderer{
		DefaultOutput: "tab=LEVEL:level,PATH:path,NODE:node,DRIVER:driver,KEY:key,KIND:kind,COMMENT:comment",
		Output:        t.Output,
		Sort:          t.Sort,
		Color:         t.Color,
		Data:          alerts,
		Colorize:      rawconfig.Colorize,
	}.Print())
	return errors.Join(errs, configAlertErrors(alerts.Items))
}

func (t *CmdObjectConfigValidate) validate(ctx context.Context, c *client.T, p naming.Path, node string) (api.ConfigAlertItems, error) {
	resp, err := c.GetInstanceConfigValidateWithResponse(ctx, node, p.Namespace, p.Kind, p.Name)
	if err != nil {
		return nil, fmt.Errorf("%s: node %s: %w", p, node, err)
	}
	switch {
	case resp.JSON200 != nil:
		return resp.JSON200.Items, nil
	case resp.JSON400 != nil:
		return nil, fmt.Errorf("%s: node %s: %s", p, node, resp.JSON400.Detail)
	case resp.JSON401 != nil:
		return nil, fmt.Errorf("%s: node %s: %s", p, node, resp.JSON401.Detail)
	case resp.JSON403 != nil:
		return nil, fmt.Errorf("%s: node %s: %s", p, node, resp.JSON403.Detail)
	case resp.JSON404 != nil:
		return nil, fmt.Errorf("%s: node %s: %s", p, node, resp.JSON404.Detail)
	case resp.JSON500 != nil:
		return nil, fmt.Errorf("%s: node %s: %s", p, node, resp.JSON500.Detail)
	default:
		return nil, fmt.Errorf("%s: node %s: unexpected response: %s", p, node, resp.Status())
	}
}

// configAlertErrors returns an error when an alert is an error, so the
// validation exits non-zero for a configuration that is not good.
func configAlertErrors(items api.ConfigAlertItems) error {
	n := 0
	for _, e := range items {
		if e.Level == api.ConfigAlertLevelError {
			n++
		}
	}
	if n > 0 {
		return fmt.Errorf("the configuration has %d error(s)", n)
	}
	return nil
}
