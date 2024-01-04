package daemonapi

import (
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/util/funcopt"
)

func newProxyClient(ctx echo.Context, nodename string, opts ...funcopt.O) (*client.T, error) {
	options := []funcopt.O{
		client.WithURL(nodename),
		client.WithAuthorization(ctx.Request().Header.Get("authorization")),
	}
	options = append(options, opts...)
	return client.New(options...)
}
