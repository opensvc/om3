package daemonapi

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonAPI) PostDaemonListenerStart(ctx echo.Context, nodename api.InPathNodeName, name api.InPathListenerName) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	return a.postDaemonSubAction(ctx, nodename, "start", fmt.Sprintf("lsnr-%s", name), func(c *client.T) (*http.Response, error) {
		return c.PostDaemonListenerStart(ctx.Request().Context(), nodename, name)
	})
}
