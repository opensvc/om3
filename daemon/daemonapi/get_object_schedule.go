package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/schedule"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonAPI) GetObjectSchedule(ctx echo.Context, nodename, namespace string, kind naming.Kind, name string) error {
	if a.localhost == nodename {
		return a.getLocalObjectSchedule(ctx, namespace, kind, name)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.GetObjectSchedule(ctx.Request().Context(), nodename, namespace, kind, name)
	})
}

func (a *DaemonAPI) getLocalObjectSchedule(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
	path, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New path", "%s", err)
	}
	if !path.Exists() {
		return JSONProblemf(ctx, http.StatusBadRequest, "No local instance", "")
	}
	o, err := object.New(path)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New node", "%s", err)
	}
	resp := api.ScheduleList{
		Kind: "ScheduleList",
	}

	type scheduler interface {
		PrintSchedule() schedule.Table
	}

	i, ok := o.(scheduler)
	if !ok {
		return ctx.JSON(http.StatusOK, resp)
	}

	for _, e := range i.PrintSchedule() {
		item := api.ScheduleItem{
			Kind: "ScheduleItem",
			Meta: api.InstanceMeta{
				Node:   e.Node,
				Object: e.Path.String(),
			},
			Data: api.Schedule{
				Action:             e.Action,
				Key:                e.Key,
				LastRunAt:          e.LastRunAt,
				LastRunFile:        e.LastRunFile,
				LastSuccessFile:    e.LastSuccessFile,
				NextRunAt:          e.NextRunAt,
				RequireCollector:   e.RequireCollector,
				RequireProvisioned: e.RequireProvisioned,
				Schedule:           e.Schedule,
			},
		}
		resp.Items = append(resp.Items, item)
	}
	return ctx.JSON(http.StatusOK, resp)
}
