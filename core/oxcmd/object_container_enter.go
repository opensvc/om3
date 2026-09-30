package oxcmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/console"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/nodeselector"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	CmdObjectContainerEnter struct {
		ObjectSelector string
		RID            string
		NodeSelector   string
	}
)

func (t *CmdObjectContainerEnter) Run(kind string) error {
	path, err := naming.ParsePath(t.ObjectSelector)
	if err != nil {
		return err
	}
	c, err := client.New()
	if err != nil {
		return err
	}
	ctx := context.Background()
	nodenames, err := t.nodes(ctx, c, path)
	if err != nil {
		return err
	}
	for _, nodename := range nodenames {
		result, err := c.Console(ctx, nodename, path, t.RID, os.Stdin, os.Stdout)
		if err != nil {
			return err
		}
		if result.Reason == console.ReasonError {
			return fmt.Errorf("%s: node %s: %s", path, nodename, result.Text)
		}
	}
	return nil
}

// nodes returns the nodes to enter the container on: the ones --node names,
// or the node the container runs on when it runs on one node only, which is
// where a container of a failover object is.
func (t *CmdObjectContainerEnter) nodes(ctx context.Context, c *client.T, path naming.Path) ([]string, error) {
	if t.NodeSelector != "" {
		return nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand()
	}
	filter := t.RID
	if filter == "" {
		filter = "container"
	}
	pathSelector := path.String()
	resp, err := c.GetResourcesWithResponse(ctx, &api.GetResourcesParams{Path: &pathSelector, Resource: &filter})
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("%s: list the container resources: %s", path, resp.Status())
	}
	nodename, err := runningNode(path, filter, resp.JSON200.Items)
	if err != nil {
		return nil, err
	}
	return []string{nodename}, nil
}

// runningNode returns the node the resources run on, when there is one and
// only one: none is nothing to enter, and several is a choice --node makes.
func runningNode(path naming.Path, filter string, items []api.ResourceItem) (string, error) {
	running := make(map[string]any)
	for _, item := range items {
		if item.Meta.EncapNode != "" || item.Data.Status == nil {
			continue
		}
		switch item.Data.Status.Status {
		case status.Up, status.StandbyUp, status.Warn:
			running[item.Meta.Node] = nil
		}
	}
	nodes := make([]string, 0, len(running))
	for node := range running {
		nodes = append(nodes, node)
	}
	sort.Strings(nodes)
	switch len(nodes) {
	case 0:
		if len(items) == 0 {
			return "", fmt.Errorf("%s: no %s resource to enter", path, filter)
		}
		return "", fmt.Errorf("%s: %s runs on no node", path, filter)
	case 1:
		return nodes[0], nil
	default:
		return "", fmt.Errorf("%s: %s runs on several nodes: name one with --node (%s)", path, filter, strings.Join(nodes, ", "))
	}
}
