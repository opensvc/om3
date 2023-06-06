package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/pubsub"
)

func (a *DaemonApi) PostObjectProgress(ctx echo.Context) error {
	var (
		payload   = api.PostObjectProgress{}
		p         path.T
		err       error
		isPartial bool
	)
	if err := ctx.Bind(&payload); err != nil {
		return JSONProblem(ctx, http.StatusBadRequest, "Failed to json decode request body", err.Error())
	}
	p, err = path.Parse(payload.Path)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid field", "path: %s", payload.Path)
	}
	state, ok := instance.MonitorStateValues[payload.State]
	if !ok {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid field", "state: %s", payload.State)
	}
	if payload.IsPartial != nil {
		isPartial = *payload.IsPartial
	}
	a.EventBus.Pub(&msgbus.ProgressInstanceMonitor{Path: p, Node: hostname.Hostname(), SessionId: payload.SessionId, State: state, IsPartial: isPartial},
		pubsub.Label{"path", p.String()}, labelApi)
	return ctx.JSON(http.StatusOK, nil)
}
