package daemonapi

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/clusternode"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/hostname"
)

func (a *DaemonAPI) proxy(ctx echo.Context, nodename string, fn func(*client.T) (*http.Response, error)) error {
	if data := node.StatusData.GetByNode(nodename); data == nil {
		return JSONProblemf(ctx, http.StatusNotFound, "node status data not found", "%s", nodename)
	}
	c, err := a.newProxyClient(ctx, nodename)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
	} else if !clusternode.Has(nodename) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid nodename", "field 'nodename' with value '%s' is not a cluster node", nodename)
	}
	if resp, err := fn(c); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
	} else {
		return ctx.Stream(resp.StatusCode, resp.Header.Get("Content-Type"), resp.Body)
	}
}

func (a *DaemonAPI) newProxyClient(ctx echo.Context, nodename string, opts ...funcopt.O) (*client.T, error) {
	options := []funcopt.O{
		client.WithURL(nodename),
	}
	authHeader := ctx.Request().Header.Get("authorization")
	if authHeader != "" {
		options = append(options, client.WithAuthorization(authHeader))
	} else if userFromContext(ctx).GetUserName() == "root" {
		grants := grantsFromContext(ctx)
		var (
			roles api.Roles
		)
		for _, role := range strings.Fields(grants.String()) {
			roles = append(roles, api.Role(role))
		}
		user := userFromContext(ctx)
		params := api.PostAuthTokenParams{
			Role: &roles,
		}
		if user, xClaims, err := userXClaims(params, user); err != nil {
			return nil, err
		} else {
			xClaims["iss"] = hostname.Hostname()
			if tk, _, err := a.JWTcreator.CreateUserToken(user, 5*time.Second, xClaims); err != nil {
				return nil, fmt.Errorf("proxy create user token: %w", err)
			} else {
				options = append(options,
					client.WithBearer(tk),
				)
			}
		}
	}
	options = append(options, opts...)
	return client.New(options...)
}
