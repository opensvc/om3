package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
)

func (a *DaemonAPI) PostObjectActionResize(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
	if v, err := assertAdmin(ctx, namespace); !v {
		return err
	}
	return a.postObjectAction(ctx, namespace, kind, name, instance.MonitorGlobalExpectResized, func(c *client.T) (*http.Response, error) {
		return c.PostObjectActionResize(ctx.Request().Context(), namespace, kind, name)
	})
}
