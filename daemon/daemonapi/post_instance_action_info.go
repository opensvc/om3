package daemonapi

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
)

func (a *DaemonAPI) PostInstanceActionInfo(ctx echo.Context, nodename, namespace string, kind naming.Kind, name string, params api.PostInstanceActionInfoParams) error {
	if v, err := assertOperator(ctx, namespace); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if a.localhost == nodename {
		return a.postLocalInstanceActionInfo(ctx, namespace, kind, name, params)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.PostInstanceActionInfo(ctx.Request().Context(), nodename, namespace, kind, name, &params)
	})
}

func (a *DaemonAPI) postLocalInstanceActionInfo(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.PostInstanceActionInfoParams) error {
	log := LogHandler(ctx, "PostInstanceActionInfo")
	var requesterSid uuid.UUID
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	log = naming.LogWithPath(log, p)
	args := []string{p.String(), "instance", "info", "--refresh"}
	if params.Rid != nil && *params.Rid != "" {
		// the info command takes the resource selector as a positional arg,
		// like its sibling group commands do.
		args = append(args, *params.Rid)
	}
	if params.SessionId != nil {
		requesterSid = *params.SessionId
	}
	if sid, err := a.apiExec(ctx, p, requesterSid, args, log); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "", "%s", err)
	} else {
		return ctx.JSON(http.StatusOK, api.InstanceActionAccepted{SessionID: sid})
	}

}
