package daemonapi

import (
	"errors"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/collector"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
	daemoncollector "github.com/opensvc/om3/v3/daemon/collector"
	"github.com/opensvc/om3/v3/daemon/daemonauth"
)

// PostInstanceCollectorAction is called by the local action process after
// it wrote the begin or the end of an instance action to report to the
// collector. It publishes the InstanceActionPending the collector speaker
// subscribes to.
//
// The request names the pending file, it does not carry it: the file is
// read here, so the announce is the same as the one the collector goroutine
// repeats until the speaker acknowledges it.
func (a *DaemonAPI) PostInstanceCollectorAction(ctx echo.Context, nodename, namespace string, kind naming.Kind, name string) error {
	if ok, err := assertStrategy(ctx, daemonauth.StrategyUX); !ok {
		return err
	}
	log := LogHandler(ctx, "PostInstanceCollectorAction")
	nodename = a.parseNodename(nodename)
	if a.localhost != nodename {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid nodename", "The collector action signal is local only: %s", nodename)
	}
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		log.Warnf("can't make path: %s", err)
		return JSONProblemf(ctx, http.StatusBadRequest, "New path", "%s", err)
	}
	var payload api.PostInstanceCollectorAction
	if err := ctx.Bind(&payload); err != nil {
		return JSONProblem(ctx, http.StatusBadRequest, "Failed to json decode request body", err.Error())
	}
	phase := collector.ActionPhase(payload.Phase)
	switch phase {
	case collector.ActionPhaseBegin, collector.ActionPhaseEnd:
	default:
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid field", "phase: %q, expected begin or end", payload.Phase)
	}

	key := collector.ActionKey(payload.ExecID, p)
	msg, err := daemoncollector.NewInstanceActionPending(daemoncollector.ActionPendingDir(), key, phase, a.localhost)
	if errors.Is(err, os.ErrNotExist) {
		return JSONProblemf(ctx, http.StatusNotFound, "Read pending action", "%s", err)
	} else if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Read pending action", "%s", err)
	}
	labels := append(daemoncollector.InstanceActionPendingLabels(msg), labelOriginAPI)
	a.Bus.Pub(msg, labels...)
	return ctx.JSON(http.StatusOK, nil)
}
