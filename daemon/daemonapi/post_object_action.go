package daemonapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/pubsub"
)

func (a *DaemonApi) PostObjectActionAbort(ctx echo.Context) error {
	return a.PostObjectAction(ctx, instance.MonitorGlobalExpectAborted)
}

func (a *DaemonApi) PostObjectActionDelete(ctx echo.Context) error {
	return a.PostObjectAction(ctx, instance.MonitorGlobalExpectDeleted)
}

func (a *DaemonApi) PostObjectActionFreeze(ctx echo.Context) error {
	return a.PostObjectAction(ctx, instance.MonitorGlobalExpectFrozen)
}

func (a *DaemonApi) PostObjectActionGiveback(ctx echo.Context) error {
	return a.PostObjectAction(ctx, instance.MonitorGlobalExpectPlaced)
}

func (a *DaemonApi) PostObjectActionProvision(ctx echo.Context) error {
	return a.PostObjectAction(ctx, instance.MonitorGlobalExpectProvisioned)
}

func (a *DaemonApi) PostObjectActionPurge(ctx echo.Context) error {
	return a.PostObjectAction(ctx, instance.MonitorGlobalExpectPurged)
}

func (a *DaemonApi) PostObjectActionStart(ctx echo.Context) error {
	return a.PostObjectAction(ctx, instance.MonitorGlobalExpectStarted)
}

func (a *DaemonApi) PostObjectActionStop(ctx echo.Context) error {
	return a.PostObjectAction(ctx, instance.MonitorGlobalExpectStopped)
}

func (a *DaemonApi) PostObjectActionUnfreeze(ctx echo.Context) error {
	return a.PostObjectAction(ctx, instance.MonitorGlobalExpectThawed)
}

func (a *DaemonApi) PostObjectActionUnprovision(ctx echo.Context) error {
	return a.PostObjectAction(ctx, instance.MonitorGlobalExpectUnprovisioned)
}

func (a *DaemonApi) PostObjectAction(ctx echo.Context, globalExpect instance.MonitorGlobalExpect) error {
	var (
		payload = api.PostObjectAction{}
		value   = instance.MonitorUpdate{}
		p       path.T
		err     error
	)
	if err := ctx.Bind(&payload); err != nil {
		return JSONProblem(ctx, http.StatusBadRequest, "Invalid Body", err.Error())
	}
	p, err = path.Parse(payload.Path)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid Body", "Invalid path: %s", payload.Path)
	}
	if instMon := instance.MonitorData.Get(p, hostname.Hostname()); instMon == nil {
		return JSONProblemf(ctx, http.StatusNotFound, "Not found", "Object does not exist: %s", payload.Path)
	}
	value = instance.MonitorUpdate{
		GlobalExpect:             &globalExpect,
		CandidateOrchestrationId: uuid.New(),
	}
	msg := msgbus.SetInstanceMonitor{
		Path:  p,
		Node:  hostname.Hostname(),
		Value: value,
		Err:   make(chan error),
	}
	a.EventBus.Pub(&msg, pubsub.Label{"path", p.String()}, labelApi)
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	var errs error
	for {
		select {
		case <-ticker.C:
			return JSONProblemf(ctx, http.StatusRequestTimeout, "set monitor", "timeout waiting for monitor commit")
		case err := <-msg.Err:
			if err != nil {
				errs = errors.Join(errs, err)
			} else if errs != nil {
				return JSONProblemf(ctx, http.StatusConflict, "set monitor", "%s", errs)
			} else {
				return ctx.JSON(http.StatusOK, api.OrchestrationQueued{
					OrchestrationId: value.CandidateOrchestrationId,
				})
			}
		case <-ctx.Request().Context().Done():
			return JSONProblemf(ctx, http.StatusGone, "set monitor", "")
		}
	}
}
