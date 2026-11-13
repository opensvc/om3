package commoncmd

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/opensvc/om3/v3/core/check"
	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/clientcontext"
	"github.com/opensvc/om3/v3/core/nodeselector"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/hostname"
)

type (
	// NodeCheckLine is a check result of a node as a listing shows it.
	NodeCheckLine struct {
		Node string `json:"node"`
		check.Line
	}
)

const (
	// nodeChecksOutput is the listing of the check results of nodes.
	nodeChecksOutput = "tab=NODE:node,TYPE:type,DRIVER:driver,INSTANCE:instance,OBJECT:path,VALUE:value_text"

	// nodeChecksSort orders the results by node, then as the results of a
	// node are.
	nodeChecksSort = "NODE,TYPE,INSTANCE"
)

// NodeChecks runs the checks of the selected nodes, each on its node through
// the api, which pushes the results to the collector, and lists them: also
// when a push failed, which the command then says.
func NodeChecks(selector string, opts OptsGlobal) error {
	selector, err := remoteNodeSelector(selector)
	if err != nil {
		return err
	}
	c, err := client.New(client.WithTimeout(0))
	if err != nil {
		return err
	}
	nodes, err := nodeselector.New(selector, nodeselector.WithClient(c)).Expand()
	if err != nil {
		return err
	}
	if len(nodes) == 0 {
		return fmt.Errorf("no node matching %s", selector)
	}
	var (
		mu    sync.Mutex
		wg    sync.WaitGroup
		errs  error
		lines = make([]NodeCheckLine, 0)
	)
	for _, node := range nodes {
		wg.Add(1)
		go func(node string) {
			defer wg.Done()
			l, err := nodeChecks(context.Background(), c, node)
			mu.Lock()
			defer mu.Unlock()
			errs = errors.Join(errs, err)
			lines = append(lines, l...)
		}(node)
	}
	wg.Wait()
	renderErr := output.Renderer{
		DefaultOutput: nodeChecksOutput,
		DefaultSort:   nodeChecksSort,
		Output:        opts.Output,
		Sort:          opts.Sort,
		Color:         opts.Color,
		Data: ListingData(opts.Output, NewList("CheckList", lines), func() any {
			return lines
		}),
		Colorize: rawconfig.Colorize,
	}.Print()
	return errors.Join(renderErr, errs)
}

// nodeChecks runs the checks of the node, and returns their results, with the
// error of their push to the collector if it failed.
func nodeChecks(ctx context.Context, c *client.T, nodename string) ([]NodeCheckLine, error) {
	resp, err := c.PostNodeActionChecksWithResponse(ctx, nodename)
	if err != nil {
		return nil, fmt.Errorf("node %s: %w", nodename, err)
	}
	if resp.JSON200 == nil {
		return nil, nodeResponseError(nodename, resp, resp.Body)
	}
	return NodeCheckLines(*resp.JSON200)
}

// NodeCheckLines returns the results of the list as a listing shows them,
// with the error of their push to the collector if it failed.
func NodeCheckLines(l api.CheckList) ([]NodeCheckLine, error) {
	lines := make([]NodeCheckLine, len(l.Items))
	for i, item := range l.Items {
		r := check.Result{
			DriverGroup: item.Data.Type,
			DriverName:  item.Data.Driver,
			Path:        item.Data.Path,
			Instance:    item.Data.Instance,
			Unit:        item.Data.Unit,
			Value:       item.Data.Value,
		}
		lines[i] = NodeCheckLine{
			Node: item.Meta.Node,
			Line: check.Line{Result: r, ValueText: check.ValueText(r.Value, r.Unit)},
		}
	}
	if l.PushError != nil {
		node := ""
		if len(l.Items) > 0 {
			node = l.Items[0].Meta.Node
		}
		if node == "" {
			return lines, fmt.Errorf("push checks: %s", *l.PushError)
		}
		return lines, fmt.Errorf("node %s: push checks: %s", node, *l.PushError)
	}
	return lines, nil
}

// remoteNodeSelector returns the selector of the nodes an action is asked of
// through the api: the selector given, or the node this runs on when there
// is none and no context names a cluster to reach.
func remoteNodeSelector(selector string) (string, error) {
	if selector != "" {
		return selector, nil
	}
	if clientcontext.IsSet() {
		return "", fmt.Errorf("remote execution requires a node selector expression")
	}
	return hostname.Hostname(), nil
}
