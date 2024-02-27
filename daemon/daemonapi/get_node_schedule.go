package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonAPI) GetNodeSchedule(ctx echo.Context, nodename string) error {
	if a.localhost == nodename {
		return a.getLocalSchedule(ctx)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.GetNodeSchedule(ctx.Request().Context(), nodename)
	})
}

func (a *DaemonAPI) getLocalSchedule(ctx echo.Context) error {
	n, err := object.NewNode()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New node", "%s", err)
	}
	resp := api.ScheduleList{
		Kind: "ScheduleList",
	}
	for _, e := range n.Schedules() {
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
