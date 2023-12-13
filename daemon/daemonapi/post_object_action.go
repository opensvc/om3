package daemonapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/pubsub"
)

func (a *DaemonApi) PostObjectActionAbort(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
	return a.postObjectAction(ctx, namespace, kind, name, instance.MonitorGlobalExpectAborted)
}

func (a *DaemonApi) PostObjectActionDelete(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
	return a.postObjectAction(ctx, namespace, kind, name, instance.MonitorGlobalExpectDeleted)
}

func (a *DaemonApi) PostObjectActionFreeze(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
	return a.postObjectAction(ctx, namespace, kind, name, instance.MonitorGlobalExpectFrozen)
}

func (a *DaemonApi) PostObjectActionGiveback(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
	return a.postObjectAction(ctx, namespace, kind, name, instance.MonitorGlobalExpectPlaced)
}

func (a *DaemonApi) PostObjectActionProvision(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
	return a.postObjectAction(ctx, namespace, kind, name, instance.MonitorGlobalExpectProvisioned)
}

func (a *DaemonApi) PostObjectActionPurge(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
	return a.postObjectAction(ctx, namespace, kind, name, instance.MonitorGlobalExpectPurged)
}

func (a *DaemonApi) PostObjectActionStart(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
	return a.postObjectAction(ctx, namespace, kind, name, instance.MonitorGlobalExpectStarted)
}

func (a *DaemonApi) PostObjectActionStop(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
	return a.postObjectAction(ctx, namespace, kind, name, instance.MonitorGlobalExpectStopped)
}

func (a *DaemonApi) PostObjectActionUnfreeze(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
	return a.postObjectAction(ctx, namespace, kind, name, instance.MonitorGlobalExpectThawed)
}

func (a *DaemonApi) PostObjectActionUnprovision(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
	return a.postObjectAction(ctx, namespace, kind, name, instance.MonitorGlobalExpectUnprovisioned)
}

func (a *DaemonApi) postObjectAction(ctx echo.Context, namespace string, kind naming.Kind, name string, globalExpect instance.MonitorGlobalExpect) error {
	var (
		value = instance.MonitorUpdate{}
		p     naming.Path
		err   error
	)
	if p, err = naming.NewPath(namespace, kind, name); err != nil {
		return JSONProblem(ctx, http.StatusBadRequest, "Invalid parameters", err.Error())
	}
	if instMon := instance.MonitorData.Get(p, a.localhost); instMon == nil {
		return JSONProblemf(ctx, http.StatusNotFound, "Not found", "Object does not exist: %s", p)
	}
	value = instance.MonitorUpdate{
		GlobalExpect:             &globalExpect,
		CandidateOrchestrationId: uuid.New(),
	}
	msg := msgbus.SetInstanceMonitor{
		Path:  p,
		Node:  a.localhost,
		Value: value,
		Err:   make(chan error),
	}
	a.EventBus.Pub(&msg, pubsub.Label{"path", p.String()}, labelApi)
	ticker := time.NewTicker(500 * time.Millisecond)
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
