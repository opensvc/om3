package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/api"
)

func (a *DaemonAPI) GetInstanceSyncSnapshots(ctx echo.Context, nodename, namespace string, kind naming.Kind, name string, params api.GetInstanceSyncSnapshotsParams) error {
	if v, err := assertGuest(ctx, namespace); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if a.localhost == nodename {
		return a.getLocalInstanceSyncSnapshots(ctx, namespace, kind, name, params)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.GetInstanceSyncSnapshots(ctx.Request().Context(), nodename, namespace, kind, name, &params)
	})
}

func (a *DaemonAPI) getLocalInstanceSyncSnapshots(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.GetInstanceSyncSnapshotsParams) error {
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "invalid path: %s", err)
	}
	if !p.Exists() {
		return JSONProblemf(ctx, http.StatusNotFound, "No local instance", "")
	}
	o, err := object.NewActor(p, object.WithVolatile(true))
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New object", "%s", err)
	}
	reqCtx := ctx.Request().Context()
	rid := "sync"
	if params.Rid != nil && *params.Rid != "" {
		rid = *params.Rid
	}
	reqCtx = actioncontext.WithRID(reqCtx, rid)
	snapshots, err := o.SyncSnapshots(reqCtx)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "List the snapshots", "%s", err)
	}
	resp := api.SyncSnapshotList{
		Kind:  "SyncSnapshotList",
		Items: make(api.SyncSnapshotItems, 0, len(snapshots)),
	}
	for _, e := range snapshots {
		resp.Items = append(resp.Items, api.SyncSnapshotItem{
			Kind: "SyncSnapshotItem",
			Meta: api.InstanceMeta{Node: a.localhost, Object: p.String()},
			Data: api.SyncSnapshot{
				Rid:       e.RID,
				Id:        e.ID,
				CreatedAt: e.CreatedAt,
				Node:      e.Node,
				Members:   append([]string{}, e.Members...),
				Location:  e.Location,
			},
		})
	}
	return ctx.JSON(http.StatusOK, resp)
}
