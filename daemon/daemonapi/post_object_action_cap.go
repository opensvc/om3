package daemonapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/pubsub"
)

// PostObjectActionCap sets the process group caps of an object and applies
// them to its running instances.
//
// The caps are written as the pg_* keywords of the configuration, through the
// checks of a configuration update, so the rbac policy, the validation and the
// claim checks weigh them, and the caps hold through a start, a failover and
// a reboot. The orchestration is queued between the checks and the write, so
// a cap the orchestration refuses is not written. It applies the
// configuration written on every node running an instance, and names it, so
// a node holding an older one waits for it rather than applying the caps it
// replaces.
//
// A request naming no cap applies the ones the configuration holds, which is
// how caps lifted or changed by hand are put back.
func (a *DaemonAPI) PostObjectActionCap(eCtx echo.Context, namespace string, kind naming.Kind, name string, params api.PostObjectActionCapParams) error {
	if v, err := assertAdmin(eCtx, namespace); !v {
		return err
	}
	switch kind {
	case naming.KindSvc, naming.KindVol:
	default:
		return JSONProblemf(eCtx, http.StatusBadRequest, "Cap", "a %s has no process group caps", kind)
	}
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(eCtx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	var sets []string
	if params.Set != nil {
		sets = *params.Set
	}
	ops, err := capOps(sets)
	if err != nil {
		return JSONProblemf(eCtx, http.StatusBadRequest, "Cap", "%s", err)
	}
	if instance.MonitorData.GetByPathAndNode(p, a.localhost) == nil {
		for nodename := range instance.MonitorData.GetByPath(p) {
			return a.proxy(eCtx, nodename, func(c *client.T) (*http.Response, error) {
				return c.PostObjectActionCap(eCtx.Request().Context(), namespace, kind, name, &params)
			})
		}
		return JSONProblemf(eCtx, http.StatusNotFound, "Not found", "Object does not exist: %s", p)
	}

	var options instance.MonitorGlobalExpectOptionsCapped
	if params.ConfigUpdatedAt != nil {
		options.ConfigUpdatedAt = *params.ConfigUpdatedAt
	}
	var pending *pendingConfigUpdate
	if len(ops) > 0 {
		log := naming.LogWithPath(LogHandler(eCtx, "postObjectActionCap"), p)
		u, err := prepareConfigUpdate(eCtx, log, p, nil, nil, ops)
		switch {
		case errors.Is(err, ErrDenied) || errors.Is(err, ErrClaimOverrun):
			return JSONProblemf(eCtx, http.StatusForbidden, "Forbidden", "%s", err)
		case errors.Is(err, ErrInvalidConfig):
			return JSONProblemf(eCtx, http.StatusBadRequest, "Cap", "%s", err)
		case err != nil:
			return JSONProblemf(eCtx, http.StatusInternalServerError, "Cap", "%s", err)
		}
		options.ConfigUpdatedAt = time.Time{}
		if u.changed {
			pending = u
		}
	}
	switch {
	case pending != nil:
		// The caps are written once the orchestration is queued, and not
		// before, so a cap refused, as one is while another orchestration
		// is in progress, is not written either: written, it would be
		// applied by the next orchestration, which did not ask for it.
		//
		// The orchestration is for a configuration as recent as this
		// moment, which is the one about to be written. The margin is the
		// clock a file is stamped with, which lags the one read here by up
		// to a kernel tick.
		options.ConfigUpdatedAt = time.Now().Add(-capClockMargin)
	case options.ConfigUpdatedAt.IsZero():
		// The caps applied are the ones this configuration holds, read
		// here, so this is the configuration the orchestration is for.
		if mtime := file.ModTime(p.ConfigFile()); !mtime.IsZero() {
			options.ConfigUpdatedAt = mtime
		}
	}

	ctx, cancel := context.WithTimeout(eCtx.Request().Context(), 500*time.Millisecond)
	defer cancel()
	globalExpect := instance.MonitorGlobalExpectCapped
	value := instance.MonitorUpdate{
		GlobalExpect:             &globalExpect,
		GlobalExpectOptions:      options,
		CandidateOrchestrationID: uuid.New(),
	}
	msg, setInstanceMonitorErr := msgbus.NewSetInstanceMonitorWithErr(ctx, p, a.localhost, value)
	a.Bus.Pub(msg, pubsub.Label{"namespace", p.Namespace}, pubsub.Label{"path", p.String()}, labelOriginAPI)
	if err := setInstanceMonitorErr.Receive(); err != nil {
		return JSONFromSetInstanceMonitorError(eCtx, &value, err)
	}
	if pending != nil {
		if _, err := pending.commit(); err != nil {
			// The orchestration waits for a configuration that is not
			// coming, and would wait for good.
			a.abortObjectOrchestration(eCtx.Request().Context(), p)
			if errors.Is(err, ErrConfigChanged) {
				return JSONProblemf(eCtx, http.StatusConflict, "Cap", "%s", err)
			}
			return JSONProblemf(eCtx, http.StatusInternalServerError, "Cap", "%s", err)
		}
		a.announceConfigFileWritten(p)
	}
	if mtime := file.ModTime(p.ConfigFile()); !mtime.IsZero() {
		eCtx.Response().Header().Add(api.HeaderLastModified, mtime.Format(time.RFC3339Nano))
	}
	return JSONFromSetInstanceMonitorError(eCtx, &value, nil)
}

// capClockMargin is how far before the request a configuration written for
// it may be stamped: the kernel stamps a file with a clock updated once a
// tick, 10ms at the slowest tick rate.
const capClockMargin = 20 * time.Millisecond

// abortObjectOrchestration aborts the orchestration of an object the local
// node queued, as "om <path> abort" does.
func (a *DaemonAPI) abortObjectOrchestration(ctx context.Context, p naming.Path) {
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	globalExpect := instance.MonitorGlobalExpectAborted
	value := instance.MonitorUpdate{
		GlobalExpect:             &globalExpect,
		CandidateOrchestrationID: uuid.New(),
	}
	msg, setInstanceMonitorErr := msgbus.NewSetInstanceMonitorWithErr(ctx, p, a.localhost, value)
	a.Bus.Pub(msg, pubsub.Label{"namespace", p.Namespace}, pubsub.Label{"path", p.String()}, labelOriginAPI)
	_ = setInstanceMonitorErr.Receive()
}

// capOps reads the set operations of a cap, refusing what is not one.
//
// A cap is the configuration update of the pg_* keywords alone, so it opens
// nothing a configuration update does not: a keyword of another name, or a
// section holding no caps, is refused rather than written, and so is an
// operator other than "=", a cap holding one value.
func capOps(sets []string) (keyop.L, error) {
	l := make(keyop.L, 0, len(sets))
	for _, s := range sets {
		op := keyop.Parse(s)
		if op.IsZero() {
			return nil, fmt.Errorf("%s: not a set operation, write [<section>.]pg_<name>=<value>", s)
		}
		if op.Op != keyop.Set {
			return nil, fmt.Errorf("%s: a cap holds one value, set it with =", s)
		}
		if !strings.HasPrefix(op.Key.BaseOption(), "pg_") {
			return nil, fmt.Errorf("%s: %s is not a process group cap, only the pg_* keywords are set here", s, op.Key.BaseOption())
		}
		if err := validCapSection(op.Key.Section); err != nil {
			return nil, fmt.Errorf("%s: %w", s, err)
		}
		if strings.ContainsAny(op.Value, "\n\r") {
			return nil, fmt.Errorf("%s: a cap is one line", op.Key)
		}
		l = append(l, *op)
	}
	return l, nil
}

// validCapSection refuses a section a cap cannot be written in: the object,
// a subset and a resource hold caps, and nothing else does.
func validCapSection(section string) error {
	switch {
	case section == "" || section == "DEFAULT":
		return nil
	case strings.HasPrefix(section, "subset#") && len(section) > len("subset#"):
		return nil
	}
	if _, err := resourceid.Parse(section); err != nil {
		return fmt.Errorf("%s is not a section holding caps: name DEFAULT, subset#<name> or a resource id", section)
	}
	return nil
}
