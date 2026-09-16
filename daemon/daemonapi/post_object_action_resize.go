package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
)

func (a *DaemonAPI) PostObjectActionResize(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.PostObjectActionResizeParams) error {
	if v, err := assertAdmin(ctx, namespace); !v {
		return err
	}
	// The size to grow to is read from the configuration by every node, and a
	// configuration write reaches them a moment after it is acknowledged. The
	// request says which configuration it is for, so a node holding an older
	// one waits for it instead of growing to the size it is replacing.
	var options instance.MonitorGlobalExpectOptionsResized
	if params.ConfigUpdatedAt != nil {
		options.ConfigUpdatedAt = *params.ConfigUpdatedAt
	}
	return a.postObjectAction(ctx, namespace, kind, name, instance.MonitorGlobalExpectResized, func(c *client.T) (*http.Response, error) {
		return c.PostObjectActionResize(ctx.Request().Context(), namespace, kind, name, &params)
	}, options)
}
