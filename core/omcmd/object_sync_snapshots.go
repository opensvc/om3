package omcmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/nodeselector"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/objectselector"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/hostname"
)

type (
	CmdObjectSyncSnapshots struct {
		OptsGlobal
		commoncmd.OptsResourceSelector
		Local        bool
		NodeSelector string
	}
)

func (t *CmdObjectSyncSnapshots) extract(selector string, c *client.T) (api.SyncSnapshotList, error) {
	if t.Local {
		return t.extractLocal(selector)
	}
	// Read locally only when the daemon is down: a request the daemon
	// refused stays refused.
	if data, err := t.extractFromDaemons(selector, c); err == nil {
		return data, nil
	} else if !client.IsDaemonDown(err) {
		return api.SyncSnapshotList{}, err
	}
	return t.extractLocal(selector)
}

func (t *CmdObjectSyncSnapshots) extractLocal(selector string) (api.SyncSnapshotList, error) {
	data := api.SyncSnapshotList{
		Kind:  "SyncSnapshotList",
		Items: make(api.SyncSnapshotItems, 0),
	}
	paths, err := objectselector.New(selector, objectselector.WithLocal(true)).MustExpand()
	if err != nil {
		return data, err
	}
	ctx := actioncontext.WithRID(context.Background(), t.RID)
	var errs error
	for _, p := range paths {
		o, err := object.NewActor(p, object.WithVolatile(true))
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}
		snapshots, err := o.SyncSnapshots(ctx)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("%s: %w", p, err))
		}
		for _, e := range snapshots {
			data.Items = append(data.Items, api.SyncSnapshotItem{
				Kind: "SyncSnapshotItem",
				Meta: api.InstanceMeta{Node: hostname.Hostname(), Object: p.String()},
				Data: api.SyncSnapshot{
					Rid:       e.RID,
					Id:        e.ID,
					CreatedAt: e.CreatedAt,
					Cluster:   &e.Cluster,
					Node:      e.Node,
					Members:   append([]string{}, e.Members...),
					Location:  e.Location,
				},
			})
		}
	}
	return data, errs
}

func (t *CmdObjectSyncSnapshots) extractFromDaemons(selector string, c *client.T) (api.SyncSnapshotList, error) {
	data := api.SyncSnapshotList{
		Kind:  "SyncSnapshotList",
		Items: make(api.SyncSnapshotItems, 0),
	}
	// The snapshots in a store are the same from any node: one node is
	// asked unless told otherwise.
	nodeSelector := t.NodeSelector
	if nodeSelector == "" {
		nodeSelector = hostname.Hostname()
	}
	nodenames, err := nodeselector.New(nodeSelector, nodeselector.WithClient(c)).Expand()
	if err != nil {
		return data, err
	}
	paths, err := objectselector.New(selector, objectselector.WithClient(c)).MustExpand()
	if err != nil {
		return data, err
	}
	var errs error
	for _, nodename := range nodenames {
		for _, p := range paths {
			d, err := t.extractFromDaemon(nodename, p, c)
			if err != nil {
				errs = errors.Join(errs, err)
				continue
			}
			data.Items = append(data.Items, d.Items...)
		}
	}
	return data, errs
}

func (t *CmdObjectSyncSnapshots) extractFromDaemon(nodename string, p naming.Path, c *client.T) (api.SyncSnapshotList, error) {
	params := api.GetInstanceSyncSnapshotsParams{}
	if t.RID != "" {
		params.Rid = &t.RID
	}
	resp, err := c.GetInstanceSyncSnapshotsWithResponse(context.Background(), nodename, p.Namespace, p.Kind, p.Name, &params)
	if err != nil {
		return api.SyncSnapshotList{}, err
	}
	switch resp.StatusCode() {
	case 200:
		return *resp.JSON200, nil
	case 401:
		return api.SyncSnapshotList{}, fmt.Errorf("%s: %s: %s", nodename, p, *resp.JSON401)
	case 403:
		return api.SyncSnapshotList{}, fmt.Errorf("%s: %s: %s", nodename, p, *resp.JSON403)
	case 404:
		return api.SyncSnapshotList{}, fmt.Errorf("%s: %s: %s", nodename, p, *resp.JSON404)
	case 500:
		return api.SyncSnapshotList{}, fmt.Errorf("%s: %s: %s", nodename, p, *resp.JSON500)
	default:
		return api.SyncSnapshotList{}, fmt.Errorf("%s: %s: unexpected statuscode: %s", nodename, p, resp.Status())
	}
}

func (t *CmdObjectSyncSnapshots) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	c, err := client.New()
	if err != nil {
		return err
	}
	data, err := t.extract(mergedSelector, c)
	return errors.Join(err, output.Renderer{
		DefaultOutput: commoncmd.SyncSnapshotsTab,
		Output:        t.Output,
		Sort:          t.Sort,
		Color:         t.Color,
		Data:          data,
		Colorize:      rawconfig.Colorize,
	}.Print())
}
