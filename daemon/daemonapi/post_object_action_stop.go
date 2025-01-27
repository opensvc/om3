package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
)

func (a *DaemonAPI) PostObjectActionStop(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
	if v, err := assertOperator(ctx, namespace); !v {
		return err
	}
	return a.postObjectAction(ctx, namespace, kind, name, instance.MonitorGlobalExpectStopped, func(c *client.T) (*http.Response, error) {
		return c.PostObjectActionStop(ctx.Request().Context(), namespace, kind, name)
	})
}
