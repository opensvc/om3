package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/schedule"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonAPI) GetInstanceSchedule(ctx echo.Context, nodename, namespace string, kind naming.Kind, name string) error {
	if v, err := assertGuest(ctx, namespace); !v {
		return err
	}
	if a.localhost == nodename {
		return a.getLocalInstanceSchedule(ctx, namespace, kind, name)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.GetInstanceSchedule(ctx.Request().Context(), nodename, namespace, kind, name)
	})
}

func (a *DaemonAPI) getLocalInstanceSchedule(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
	path, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New path", "%s", err)
	}
	if !path.Exists() {
		return JSONProblemf(ctx, http.StatusNotFound, "No local instance", "")
	}
	table := schedule.TableData.GetByPath(path)
	if table == nil {
		return JSONProblemf(ctx, http.StatusNotFound, "No schedule table cached", "")
	}
	resp := api.ScheduleList{
		Kind: "ScheduleList",
	}

	for _, e := range *table {
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
				Require:            e.Require,
				RequireCollector:   e.RequireCollector,
				RequireProvisioned: e.RequireProvisioned,
				Schedule:           e.Schedule,
			},
		}
		resp.Items = append(resp.Items, item)
	}
	return ctx.JSON(http.StatusOK, resp)
}
