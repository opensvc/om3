package daemonapi

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/proc"
)

func (a *DaemonAPI) GetDaemonProcess(ctx echo.Context, nodename string, params api.GetDaemonProcessParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}

	nodename = a.parseNodename(nodename)
	if a.localhost != nodename {
		return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.GetDaemonProcess(ctx.Request().Context(), nodename, &params)
		})
	}
	return a.getLocalDaemonProcess(ctx, params)
}

func (a *DaemonAPI) getLocalDaemonProcess(ctx echo.Context, params api.GetDaemonProcessParams) error {
	var subFilter []string
	if params.Sub != nil && *params.Sub != "" {
		subFilter = strings.Split(*params.Sub, ",")
	}
	items := procToProcessItem(proc.List(subFilter))
	return ctx.JSON(http.StatusOK, api.ProcessList{Kind: "ProcessList", Items: items})
}

func procToProcessItem(elements []proc.T) api.ProcessItems {
	var res api.ProcessItems
	for _, item := range elements {
		res = append(res, api.ProcessItem{
			Pid:          item.Pid,
			Node:         item.Node,
			Object:       item.Object,
			Sid:          item.Sid,
			StartedAt:    item.StartedAt,
			Elapsed:      item.Elapsed,
			GlobalExpect: item.GlobalExpect,
			Sub:          item.Sub,
			Desc:         item.Desc,
		})
	}
	return res
}
