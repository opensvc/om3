package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
)

func (a *DaemonAPI) PostObjectActionPurge(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
	if _, err := assertAdmin(ctx, namespace); err != nil {
		return err
	}
	return a.postObjectAction(ctx, namespace, kind, name, instance.MonitorGlobalExpectPurged, func(c *client.T) (*http.Response, error) {
		return c.PostObjectActionPurge(ctx.Request().Context(), namespace, kind, name)
	})
}
