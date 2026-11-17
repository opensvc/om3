package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
)

func (a *DaemonAPI) PostObjectActionUnprovision(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.PostObjectActionUnprovisionParams) error {
	if v, err := assertAdmin(ctx, namespace); !v {
		return err
	}
	// Force goes on without the instances of the nodes away, which the
	// orchestration does not wait for.
	options := instance.MonitorGlobalExpectOptionsUnprovisioned{}
	if params.Force != nil && *params.Force {
		options.Force = true
	}
	return a.postObjectAction(ctx, namespace, kind, name, instance.MonitorGlobalExpectUnprovisioned, func(c *client.T) (*http.Response, error) {
		return c.PostObjectActionUnprovision(ctx.Request().Context(), namespace, kind, name, &params)
	}, options)
}
