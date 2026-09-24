package daemonapi

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/env"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
)

// PostNodeActionRegister registers the node named in the path on the
// collector.
//
// The collector mints a registration id for a nodename, so a node cannot
// register on behalf of another: the credentials travel to the node, which
// registers itself, and a request for a peer is proxied to it.
func (a *DaemonAPI) PostNodeActionRegister(ctx echo.Context, nodename api.InPathNodeName, params api.PostNodeActionRegisterParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	var payload api.PostNodeActionRegisterRequest
	if err := ctx.Bind(&payload); err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "%s", err)
	}
	if err := nodeRegisterCredentialError(payload); err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "%s", err)
	}
	nodename = a.parseNodename(nodename)
	if nodename == a.localhost {
		return a.localNodeActionRegister(ctx, payload, params)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.PostNodeActionRegister(ctx.Request().Context(), nodename, &params, payload)
	})
}

func (a *DaemonAPI) localNodeActionRegister(ctx echo.Context, payload api.PostNodeActionRegisterRequest, params api.PostNodeActionRegisterParams) error {
	log := LogHandler(ctx, "PostNodeActionRegister")
	var requesterSessionID uuid.UUID
	if params.SessionID != nil {
		requesterSessionID = *params.SessionID
	}
	args := nodeRegisterArgs(payload)
	varEnv := nodeRegisterVarEnv(payload)
	if sessionID, execID, err := a.apiExec(ctx, naming.Path{}, requesterSessionID, args, log, varEnv...); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "", "%s", err)
	} else {
		return ctx.JSON(http.StatusOK, api.NodeActionAccepted{SessionID: sessionID, ExecID: execID})
	}
}

// nodeRegisterCredentialError returns the reason the payload credentials are
// refused, and nil when they are usable.
//
// A user without a password would reach the password prompt of the register
// command, on a daemon that has no terminal to prompt on. A password without
// a user names nobody to authenticate as, and would be silently dropped: the
// node would register with the id it already holds, which is not what the
// caller asked for.
func nodeRegisterCredentialError(payload api.PostNodeActionRegisterRequest) error {
	user := deref(payload.User)
	password := deref(payload.Password)
	switch {
	case user != "" && password == "":
		return fmt.Errorf("field 'user' without field 'password'")
	case user == "" && password != "":
		return fmt.Errorf("field 'password' without field 'user'")
	}
	return nil
}

// nodeRegisterArgs returns the arguments of the node register to fork.
//
// The credentials are not among them: a command line is readable by any user
// through /proc/<pid>/cmdline, and the command string is published on the bus
// and kept in the exec store.
func nodeRegisterArgs(payload api.PostNodeActionRegisterRequest) []string {
	args := []string{"node", "register"}
	if app := deref(payload.App); app != "" {
		args = append(args, "--app", app)
	}
	return args
}

// nodeRegisterVarEnv returns the environment entries handing the collector
// credentials to the forked command, and nothing when the payload carries
// none: the node then registers with the id it already holds.
func nodeRegisterVarEnv(payload api.PostNodeActionRegisterRequest) []string {
	user := deref(payload.User)
	if user == "" {
		return nil
	}
	return []string{env.CollectorCredentialVar + "=" + user + ":" + deref(payload.Password)}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
