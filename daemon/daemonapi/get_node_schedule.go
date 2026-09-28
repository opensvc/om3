package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/schedule"
	"github.com/opensvc/om3/v3/daemon/api"
)

func (a *DaemonAPI) GetNodeSchedule(ctx echo.Context, nodename string) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if a.localhost == nodename {
		return a.getLocalSchedule(ctx)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.GetNodeSchedule(ctx.Request().Context(), nodename)
	})
}

func (a *DaemonAPI) getLocalSchedule(ctx echo.Context) error {
	table := schedule.TableData.GetByPath(naming.Path{})
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
	sortScheduleItems(resp.Items)
	return ctx.JSON(http.StatusOK, resp)
}
