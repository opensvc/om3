package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/pubsub"
)

func (a *DaemonAPI) PostInstanceProgress(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
	var (
		payload   = api.PostInstanceProgress{}
		isPartial bool
	)
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		JSONProblem(ctx, http.StatusBadRequest, "Invalid parameters", err.Error())
		return err
	}
	if err := ctx.Bind(&payload); err != nil {
		JSONProblem(ctx, http.StatusBadRequest, "Failed to json decode request body", err.Error())
		return err
	}
	state, ok := instance.MonitorStateValues[payload.State]
	if !ok {
		JSONProblemf(ctx, http.StatusBadRequest, "Invalid field", "state: %s", payload.State)
		return err
	}
	if payload.IsPartial != nil {
		isPartial = *payload.IsPartial
	}
	a.EventBus.Pub(&msgbus.ProgressInstanceMonitor{Path: p, Node: a.localhost, SessionID: payload.SessionID, State: state, IsPartial: isPartial},
		pubsub.Label{"path", p.String()}, labelAPI)
	return ctx.JSON(http.StatusOK, nil)
}
