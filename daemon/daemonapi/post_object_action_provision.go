package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
)

func (a *DaemonAPI) PostObjectActionProvision(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
	return a.postObjectAction(ctx, namespace, kind, name, instance.MonitorGlobalExpectProvisioned, func(c *client.T) (*http.Response, error) {
		return c.PostObjectActionProvision(ctx.Request().Context(), namespace, kind, name)
	})
}
