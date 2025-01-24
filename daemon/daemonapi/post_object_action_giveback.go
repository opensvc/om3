package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
)

func (a *DaemonAPI) PostObjectActionGiveback(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
	if _, err := assertOperator(ctx, namespace); err != nil {
		return err
	}
	return a.postObjectAction(ctx, namespace, kind, name, instance.MonitorGlobalExpectPlaced, func(c *client.T) (*http.Response, error) {
		return c.PostObjectActionGiveback(ctx.Request().Context(), namespace, kind, name)
	})
}
