package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/pubsub"
)

func (a *DaemonAPI) PostInstanceClear(ctx echo.Context, nodename, namespace string, kind naming.Kind, name string) error {
	if _, err := assertOperator(ctx, namespace); err != nil {
		return err
	}
	if a.localhost == nodename {
		return a.postLocalInstanceClear(ctx, namespace, kind, name)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.PostInstanceClear(ctx.Request().Context(), nodename, namespace, kind, name)
	})
}

func (a *DaemonAPI) postLocalInstanceClear(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	state := instance.MonitorStateIdle
	instMonitor := instance.MonitorUpdate{
		State: &state,
	}
	msg := msgbus.SetInstanceMonitor{
		Path:  p,
		Node:  a.localhost,
		Value: instMonitor,
	}
	a.EventBus.Pub(&msg, pubsub.Label{"path", p.String()}, labelAPI)
	return ctx.JSON(http.StatusOK, nil)
}
