package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/msgbus"
)

func (a *DaemonAPI) PostNodeClear(ctx echo.Context) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	state := node.MonitorStateIdle
	a.Publisher.Pub(&msgbus.SetNodeMonitor{Node: a.localhost, Value: node.MonitorUpdate{State: &state}},
		labelOriginAPI)
	return ctx.JSON(http.StatusOK, nil)
}
