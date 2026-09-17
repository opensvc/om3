package daemonapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/pool"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/pubsub"
	"github.com/opensvc/om3/v3/util/sizeconv"
)

// PostObjectActionResize grows an object on every node holding an instance.
//
// The size asked for is written to the object configuration, which is the size
// every node converges to, and only then is the orchestration queued. Both
// happen here rather than in the client: the configuration is on the cluster
// nodes, and the claim the namespace has on the pool is the cluster's to
// enforce, so a client is not the one to decide the grow fits.
//
// A request naming no size asks for the size already configured, which is how
// a resize that stopped part way is finished.
func (a *DaemonAPI) PostObjectActionResize(eCtx echo.Context, namespace string, kind naming.Kind, name string, params api.PostObjectActionResizeParams) error {
	if v, err := assertAdmin(eCtx, namespace); !v {
		return err
	}
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(eCtx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	if instMon := instance.MonitorData.GetByPathAndNode(p, a.localhost); instMon != nil {
		var payload api.PostObjectActionResize
		if err := eCtx.Bind(&payload); err != nil {
			return JSONProblem(eCtx, http.StatusBadRequest, "Invalid Body", err.Error())
		}

		// The size to grow to is read from the configuration by every node,
		// and a configuration write reaches them a moment after it is
		// acknowledged. The orchestration says which configuration it is for,
		// so a node holding an older one waits for it instead of growing to
		// the size it is replacing.
		var options instance.MonitorGlobalExpectOptionsResized
		if params.ConfigUpdatedAt != nil {
			options.ConfigUpdatedAt = *params.ConfigUpdatedAt
		}
		if payload.Size != nil {
			updatedAt, code, err := writeResizeTarget(eCtx, p, *payload.Size)
			if err != nil {
				return JSONProblemf(eCtx, code, "Resize", "%s", err)
			}
			options.ConfigUpdatedAt = updatedAt
		}

		ctx, cancel := context.WithTimeout(eCtx.Request().Context(), 500*time.Millisecond)
		defer cancel()

		globalExpect := instance.MonitorGlobalExpectResized
		value := instance.MonitorUpdate{
			GlobalExpect:             &globalExpect,
			GlobalExpectOptions:      options,
			CandidateOrchestrationID: uuid.New(),
		}

		msg, setInstanceMonitorErr := msgbus.NewSetInstanceMonitorWithErr(ctx, p, a.localhost, value)

		a.Bus.Pub(msg, pubsub.Label{"namespace", p.Namespace}, pubsub.Label{"path", p.String()}, labelOriginAPI)

		return JSONFromSetInstanceMonitorError(eCtx, &value, setInstanceMonitorErr.Receive())
	}
	for nodename := range instance.MonitorData.GetByPath(p) {
		return a.proxy(eCtx, nodename, func(c *client.T) (*http.Response, error) {
			return c.PostObjectActionResizeWithBody(eCtx.Request().Context(), namespace, kind, name, &params, eCtx.Request().Header.Get("Content-Type"), eCtx.Request().Body)
		})
	}
	return JSONProblemf(eCtx, http.StatusNotFound, "Not found", "Object does not exist: %s", p)
}

// writeResizeTarget writes the size the object is to hold and answers the
// timestamp the configuration then carries, along with the status code the
// failure to write it is, when it fails.
func writeResizeTarget(eCtx echo.Context, p naming.Path, size string) (time.Time, int, error) {
	var updatedAt time.Time
	change, err := sizeconv.ParseChange(size)
	if err != nil {
		return updatedAt, http.StatusBadRequest, err
	}
	to, code, err := resizeTarget(p, change)
	if err != nil {
		return updatedAt, code, err
	}
	if code, err := resizeClaimFits(eCtx.Request().Context(), p, to); err != nil {
		return updatedAt, code, err
	}
	sets := keyop.ParseOps([]string{fmt.Sprintf("size=%d", to)})
	if err := refuseSizeWhileResizing(p, sets); err != nil {
		return updatedAt, http.StatusConflict, err
	}
	log := naming.LogWithPath(LogHandler(eCtx, "postObjectActionResize"), p)
	if _, err := configUpdate(eCtx, log, p, nil, nil, sets); errors.Is(err, ErrDenied) {
		return updatedAt, http.StatusForbidden, err
	} else if err != nil {
		return updatedAt, http.StatusInternalServerError, err
	}
	if mtime := file.ModTime(p.ConfigFile()); !mtime.IsZero() {
		updatedAt = mtime
	}
	return updatedAt, http.StatusOK, nil
}

// resizeTarget is the size to reach.
//
// A resize only grows, and the size already configured is the target, so
// asking for it again finishes a resize that stopped part way and asking for
// less is refused.
func resizeTarget(p naming.Path, change sizeconv.Change) (int64, int, error) {
	from, err := configuredSize(p)
	switch {
	case err != nil && change.IsRelative:
		return 0, http.StatusBadRequest, fmt.Errorf("%s: an amount to add is resolved against the configured size: %w", p, err)
	case err != nil:
		// Nothing to resolve against and nothing to compare to. The daemons
		// still refuse what they cannot do.
		return change.Value, http.StatusOK, nil
	}
	to := change.Resolve(from)
	if to < from {
		return 0, http.StatusBadRequest, fmt.Errorf("%s is configured to hold %s, and a resize only grows",
			p, sizeconv.BSizeCompact(float64(from)))
	}
	// to == from is not refused: the configuration is the target, and asking
	// for it again finishes a resize that stopped part way. Every link that
	// holds it already is skipped, so an object that reached it has nothing
	// to do.
	return to, http.StatusOK, nil
}

// resizeClaimFits refuses a grow the namespace has no room for in the pool
// serving the object. An object served by no pool is claimed from nothing and
// capped by nothing.
//
// Growing is claiming more of the pool, so it is checked the way an allocation
// is. What the object already holds is counted in, so only what it asks for on
// top has to fit.
func resizeClaimFits(ctx context.Context, p naming.Path, to int64) (int, error) {
	type poolNamer interface {
		PoolName() (string, error)
	}
	o, err := object.New(p, object.WithVolatile(true))
	if err != nil {
		return http.StatusInternalServerError, err
	}
	i, ok := o.(poolNamer)
	if !ok {
		return http.StatusOK, nil
	}
	poolName, err := i.PoolName()
	if err != nil || poolName == "" {
		return http.StatusOK, nil
	}
	from, err := configuredSize(p)
	if err != nil || to <= from {
		return http.StatusOK, nil
	}
	ok, why, err := pool.ClaimFits(ctx, p.Namespace, poolName, to-from)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if !ok {
		return http.StatusForbidden, fmt.Errorf("%s is served by the %s pool, and %s", p, poolName, why)
	}
	return http.StatusOK, nil
}

// configuredSize is the size the object is asked to be.
func configuredSize(p naming.Path) (int64, error) {
	type configuredSizer interface {
		ConfiguredSize() (int64, error)
	}
	o, err := object.New(p, object.WithVolatile(true))
	if err != nil {
		return 0, err
	}
	i, ok := o.(configuredSizer)
	if !ok {
		return 0, fmt.Errorf("a %s has no configured size", p.Kind)
	}
	return i.ConfiguredSize()
}
