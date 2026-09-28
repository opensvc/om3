package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/schedule"
	"github.com/opensvc/om3/v3/daemon/api"
)

func (a *DaemonAPI) GetInstanceSchedule(ctx echo.Context, nodename, namespace string, kind naming.Kind, name string) error {
	if v, err := assertGuest(ctx, namespace); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
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
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "invalid path: %s", err)
	}
	if !path.Exists() {
		return JSONProblemf(ctx, http.StatusNotFound, "No local instance", "")
	}
	table := schedule.TableData.GetByPath(path)
	if table == nil {
		return JSONProblemf(ctx, http.StatusNotFound, "No schedule table cached", "")
	}
	resp := api.ScheduleList{
		Kind:  "ScheduleList",
		Items: make(api.ScheduleItems, 0, len(*table)),
	}

	for _, e := range *table {
		resp.Items = append(resp.Items, scheduleItem(e))
	}
	return ctx.JSON(http.StatusOK, resp)
}
