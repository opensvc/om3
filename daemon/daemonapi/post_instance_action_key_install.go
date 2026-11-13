package daemonapi

import (
	"net/http"
	"slices"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
)

// PostInstanceActionKeyInstall installs again the keys of a datastore in the
// volumes of the node projecting them, as the daemon does when a key changes.
// It writes no key, and refreshes what the instances of the node install, as
// an instance install does: operator.
func (a *DaemonAPI) PostInstanceActionKeyInstall(ctx echo.Context, nodename api.InPathNodeName, namespace api.InPathNamespace, kind api.InPathKind, name api.InPathName, params api.PostInstanceActionKeyInstallParams) error {
	if v, err := assertOperator(ctx, namespace); !v {
		return err
	}
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	if !slices.Contains(naming.KindDataStore, p.Kind) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s is not a datastore, which holds keys", p)
	}
	nodename = a.parseNodename(nodename)
	if nodename != a.localhost {
		return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.PostInstanceActionKeyInstall(ctx.Request().Context(), nodename, namespace, kind, name, &params)
		})
	}
	log := naming.LogWithPath(LogHandler(ctx, "PostInstanceActionKeyInstall"), p)
	args := []string{p.String(), "key", "install"}
	if params.Name != nil && *params.Name != "" {
		args = append(args, *params.Name)
	}
	var requesterSessionID uuid.UUID
	if params.SessionID != nil {
		requesterSessionID = *params.SessionID
	}
	sessionID, execID, err := a.apiExec(ctx, p, requesterSessionID, args, log)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "", "%s", err)
	}
	return ctx.JSON(http.StatusOK, api.InstanceActionAccepted{SessionID: sessionID, ExecID: execID})
}
