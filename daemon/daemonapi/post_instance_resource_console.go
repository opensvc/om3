package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/console"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/api"
	daemonconsole "github.com/opensvc/om3/v3/daemon/console"
	"github.com/opensvc/om3/v3/util/key"
)

// PostInstanceResourceConsole issues the ticket a console session of the
// resource is opened with.
//
// The api decides who may open a console, and the session is not served
// through it: the ticket is presented to the console listener, which hands
// the session to a process of its own.
func (a *DaemonAPI) PostInstanceResourceConsole(ctx echo.Context, nodename, namespace string, kind naming.Kind, name string, params api.PostInstanceResourceConsoleParams) error {
	if v, err := assertAdmin(ctx, namespace); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if a.localhost == nodename {
		return a.localInstanceResourceConsole(ctx, namespace, kind, name, params)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.PostInstanceResourceConsole(ctx.Request().Context(), nodename, namespace, kind, name, &params)
	})
}

func (a *DaemonAPI) localInstanceResourceConsole(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.PostInstanceResourceConsoleParams) error {
	path, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New path", "%s", err)
	}
	if !path.Exists() {
		return JSONProblemf(ctx, http.StatusNotFound, "No local instance", "")
	}
	target := console.Target{
		Node: a.localhost,
		Path: path.String(),
		Kind: console.KindTTY,
	}
	if params.Rid != nil {
		target.RID = *params.Rid
	}
	ticket, err := console.NewTicket(userFromContext(ctx).Username, a.localhost, target)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New console ticket", "%s", err)
	}
	signed, expiredAt, err := a.JWTcreator.CreateToken(console.TicketDuration, ticket.Claims())
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Sign console ticket", "%s", err)
	}
	resp := api.ConsoleTicket{
		Ticket:    signed,
		Port:      daemonconsole.Port(),
		ExpiredAt: expiredAt,
	}
	if node, err := object.NewNode(); err == nil {
		if url := node.MergedConfig().GetString(key.New("console", "url")); url != "" {
			resp.Url = &url
		}
	}
	return ctx.JSON(http.StatusCreated, resp)
}
