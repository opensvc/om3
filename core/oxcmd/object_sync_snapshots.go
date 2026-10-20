package oxcmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/nodeselector"
	"github.com/opensvc/om3/v3/core/objectselector"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	CmdObjectSyncSnapshots struct {
		OptsGlobal
		commoncmd.OptsResourceSelector
		NodeSelector string
	}
)

func (t *CmdObjectSyncSnapshots) Run(kind string) error {
	c, err := client.New()
	if err != nil {
		return err
	}
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	paths, err := objectselector.New(mergedSelector, objectselector.WithClient(c)).MustExpand()
	if err != nil {
		return err
	}
	// The snapshots in a store are the same from any node: the node of
	// the context is asked unless told otherwise.
	nodenames := []string{api.AliasLocalhost}
	if t.NodeSelector != "" {
		if nodenames, err = nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand(); err != nil {
			return err
		}
	}
	data := api.SyncSnapshotList{
		Kind:  "SyncSnapshotList",
		Items: make(api.SyncSnapshotItems, 0),
	}
	var errs error
	for _, nodename := range nodenames {
		for _, p := range paths {
			items, err := t.extract(c, nodename, p)
			if err != nil {
				errs = errors.Join(errs, err)
				continue
			}
			data.Items = append(data.Items, items...)
		}
	}
	return errors.Join(errs, output.Renderer{
		DefaultOutput: commoncmd.SyncSnapshotsTab,
		Output:        t.Output,
		Sort:          t.Sort,
		Color:         t.Color,
		Data:          data,
		Colorize:      rawconfig.Colorize,
	}.Print())
}

func (t *CmdObjectSyncSnapshots) extract(c *client.T, nodename string, p naming.Path) (api.SyncSnapshotItems, error) {
	params := api.GetInstanceSyncSnapshotsParams{}
	if t.RID != "" {
		params.Rid = &t.RID
	}
	resp, err := c.GetInstanceSyncSnapshotsWithResponse(context.Background(), nodename, p.Namespace, p.Kind, p.Name, &params)
	if err != nil {
		return nil, err
	}
	switch {
	case resp.JSON200 != nil:
		return resp.JSON200.Items, nil
	case resp.JSON401 != nil:
		return nil, fmt.Errorf("%s: %s: %s", nodename, p, *resp.JSON401)
	case resp.JSON403 != nil:
		return nil, fmt.Errorf("%s: %s: %s", nodename, p, *resp.JSON403)
	case resp.JSON404 != nil:
		return nil, fmt.Errorf("%s: %s: %s", nodename, p, *resp.JSON404)
	case resp.JSON500 != nil:
		return nil, fmt.Errorf("%s: %s: %s", nodename, p, *resp.JSON500)
	default:
		return nil, fmt.Errorf("%s: %s: unexpected response: %s", nodename, p, resp.Status())
	}
}
