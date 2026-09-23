package daemonapi

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/api"
)

// registerTimeout bounds the registration of a node on the collector. It
// covers the collector round-trip and the initial asset, package and disk
// discovery the registration sends, so it is far longer than the timeout of
// a single collector request.
const registerTimeout = 5 * time.Minute

// PostNodeActionRegister registers the node named in the path on the
// collector.
//
// The collector mints a registration id for a nodename, so a node cannot
// register on behalf of another: the credentials travel to the node, which
// registers itself, and a request for a peer is proxied to it.
//
// The work is done in process rather than by running the register command,
// because the password would then be readable in the process table and kept
// in the exec session records.
func (a *DaemonAPI) PostNodeActionRegister(ctx echo.Context, nodename api.InPathNodeName) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	var payload api.PostNodeActionRegisterRequest
	if err := ctx.Bind(&payload); err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "%s", err)
	}
	nodename = a.parseNodename(nodename)
	if nodename == a.localhost {
		return a.localNodeActionRegister(ctx, payload)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.PostNodeActionRegister(ctx.Request().Context(), nodename, payload)
	})
}

func (a *DaemonAPI) localNodeActionRegister(ctx echo.Context, payload api.PostNodeActionRegisterRequest) error {
	log := LogHandler(ctx, "PostNodeActionRegister")
	var user, password, app string
	if payload.User != nil {
		user = *payload.User
	}
	if payload.Password != nil {
		password = *payload.Password
	}
	if payload.App != nil {
		app = *payload.App
	}
	n, err := object.NewNode()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New node", "%s", err)
	}
	requestCtx, cancel := context.WithTimeout(ctx.Request().Context(), registerTimeout)
	defer cancel()

	log.Infof("register on the collector")
	if err := n.Register(requestCtx, user, password, app); err != nil {
		// The error can name the collector url and the user, never the
		// password: Register is handed it and does not echo it.
		log.Errorf("register on the collector: %s", err)
		return JSONProblemf(ctx, http.StatusInternalServerError, "Register", "%s", err)
	}
	log.Infof("registered on the collector")
	return ctx.NoContent(http.StatusNoContent)
}
