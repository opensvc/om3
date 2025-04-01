package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
)

func (a *DaemonAPI) PutNodeConfigFile(ctx echo.Context, nodename string) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if nodename == a.localhost {
		return a.writeNodeConfigFile(ctx, nodename)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.PutNodeConfigFileWithBody(ctx.Request().Context(), nodename, "application/octet-stream", ctx.Request().Body)
	})
}
