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

func (a *DaemonApi) PostSvcActionAbort(ctx echo.Context, namespace, name string) error {
	return a.PostObjectAction(ctx, namespace, "svc", name, instance.MonitorGlobalExpectAborted)
}

func (a *DaemonApi) PostVolActionAbort(ctx echo.Context, namespace, name string) error {
	return a.PostObjectAction(ctx, namespace, "vol", name, instance.MonitorGlobalExpectAborted)
}

func (a *DaemonApi) PostSvcActionDelete(ctx echo.Context, namespace, name string) error {
	return a.PostObjectAction(ctx, namespace, "svc", name, instance.MonitorGlobalExpectDeleted)
}

func (a *DaemonApi) PostVolActionDelete(ctx echo.Context, namespace, name string) error {
	return a.PostObjectAction(ctx, namespace, "vol", name, instance.MonitorGlobalExpectDeleted)
}

func (a *DaemonApi) PostCfgActionDelete(ctx echo.Context, namespace, name string) error {
	return a.PostObjectAction(ctx, namespace, "cfg", name, instance.MonitorGlobalExpectDeleted)
}

func (a *DaemonApi) PostSecActionDelete(ctx echo.Context, namespace, name string) error {
	return a.PostObjectAction(ctx, namespace, "sec", name, instance.MonitorGlobalExpectDeleted)
}

func (a *DaemonApi) PostUsrActionDelete(ctx echo.Context, namespace, name string) error {
	return a.PostObjectAction(ctx, namespace, "usr", name, instance.MonitorGlobalExpectDeleted)
}

func (a *DaemonApi) PostSvcActionFreeze(ctx echo.Context, namespace, name string) error {
	return a.PostObjectAction(ctx, namespace, "svc", name, instance.MonitorGlobalExpectFrozen)
}

func (a *DaemonApi) PostVolActionFreeze(ctx echo.Context, namespace, name string) error {
	return a.PostObjectAction(ctx, namespace, "vol", name, instance.MonitorGlobalExpectFrozen)
}

func (a *DaemonApi) PostSvcActionGiveback(ctx echo.Context, namespace, name string) error {
	return a.PostObjectAction(ctx, namespace, "svc", name, instance.MonitorGlobalExpectPlaced)
}

func (a *DaemonApi) PostSvcActionProvision(ctx echo.Context, namespace, name string) error {
	return a.PostObjectAction(ctx, namespace, "svc", name, instance.MonitorGlobalExpectProvisioned)
}

func (a *DaemonApi) PostVolActionProvision(ctx echo.Context, namespace, name string) error {
	return a.PostObjectAction(ctx, namespace, "vol", name, instance.MonitorGlobalExpectProvisioned)
}

func (a *DaemonApi) PostSvcActionPurge(ctx echo.Context, namespace, name string) error {
	return a.PostObjectAction(ctx, namespace, "svc", name, instance.MonitorGlobalExpectPurged)
}

func (a *DaemonApi) PostVolActionPurge(ctx echo.Context, namespace, name string) error {
	return a.PostObjectAction(ctx, namespace, "vol", name, instance.MonitorGlobalExpectPurged)
}

func (a *DaemonApi) PostSvcActionStart(ctx echo.Context, namespace, name string) error {
	return a.PostObjectAction(ctx, namespace, "svc", name, instance.MonitorGlobalExpectStarted)
}

func (a *DaemonApi) PostSvcActionStop(ctx echo.Context, namespace, name string) error {
	return a.PostObjectAction(ctx, namespace, "svc", name, instance.MonitorGlobalExpectStopped)
}

func (a *DaemonApi) PostSvcActionUnfreeze(ctx echo.Context, namespace, name string) error {
	return a.PostObjectAction(ctx, namespace, "svc", name, instance.MonitorGlobalExpectThawed)
}

func (a *DaemonApi) PostVolActionUnfreeze(ctx echo.Context, namespace, name string) error {
	return a.PostObjectAction(ctx, namespace, "vol", name, instance.MonitorGlobalExpectThawed)
}

func (a *DaemonApi) PostSvcActionUnprovision(ctx echo.Context, namespace, name string) error {
	return a.PostObjectAction(ctx, namespace, "svc", name, instance.MonitorGlobalExpectUnprovisioned)
}

func (a *DaemonApi) PostVolActionUnprovision(ctx echo.Context, namespace, name string) error {
	return a.PostObjectAction(ctx, namespace, "vol", name, instance.MonitorGlobalExpectUnprovisioned)
}

func (a *DaemonApi) PostObjectAction(ctx echo.Context, namespace, kind, name string, globalExpect instance.MonitorGlobalExpect) error {
	var (
		value = instance.MonitorUpdate{}
		p     path.T
		err   error
	)
	if p, err = path.New(name, namespace, kind); err != nil {
		return JSONProblem(ctx, http.StatusBadRequest, "Invalid parameters", err.Error())
	}
	if instMon := instance.MonitorData.Get(p, hostname.Hostname()); instMon == nil {
		return JSONProblemf(ctx, http.StatusNotFound, "Not found", "Object does not exist: %s", p)
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
