package daemonapi

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/wakeonlan"
)

func (a *DaemonAPI) PostNodeActionWake(ctx echo.Context, nodename string, params api.PostNodeActionWakeParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	if err := validateNodeActionWakeParams(params, clusternode.Has); err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	nodename = a.parseNodename(nodename)
	if nodename == a.localhost {
		return a.localNodeActionWake(ctx, params)
	}
	// The peer answers once the wake ended, later than a client allows an
	// answer by default.
	return a.proxyLong(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.PostNodeActionWake(ctx.Request().Context(), nodename, &params)
	})
}

func validateNodeActionWakeParams(params api.PostNodeActionWakeParams, isClusterNode func(string) bool) error {
	var errs error
	if params.Target == "" {
		errs = errors.Join(errs, errors.New("no target node"))
	} else if !isClusterNode(params.Target) {
		errs = errors.Join(errs, fmt.Errorf("target %s is not a cluster node", params.Target))
	}
	if params.Mac != nil {
		for _, s := range *params.Mac {
			if _, err := wakeonlan.ParseMAC(s); err != nil {
				errs = errors.Join(errs, err)
			}
		}
	}
	return errs
}

// wakeTimeout bounds the wake run from the node: a request to the collector
// and a few magic packets.
var wakeTimeout = time.Minute

// localNodeActionWake runs the wake and answers once it ended, so the client
// learns whether this node failed to wake target, and why, instead of
// waiting for a node nobody wakes.
func (a *DaemonAPI) localNodeActionWake(ctx echo.Context, params api.PostNodeActionWakeParams) error {
	log := LogHandler(ctx, "PostNodeActionWake")
	var requesterSessionID uuid.UUID
	args := []string{"node", "wake", "--local", "--node=" + params.Target}
	if params.Mac != nil && len(*params.Mac) > 0 {
		args = append(args, "--mac="+strings.Join(*params.Mac, ","))
	}
	if params.SessionID != nil {
		requesterSessionID = *params.SessionID
	}
	if sessionID, execID, err := a.apiExecWait(ctx, naming.Path{}, requesterSessionID, args, log, wakeTimeout); err != nil {
		detail := strings.TrimPrefix(err.Error(), params.Target+": ")
		return JSONProblemf(ctx, http.StatusInternalServerError, "Wake failed", "%s", detail)
	} else {
		return ctx.JSON(http.StatusOK, api.NodeActionAccepted{SessionID: sessionID, ExecID: execID})
	}
}
