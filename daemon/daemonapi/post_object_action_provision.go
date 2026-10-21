package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
)

func (a *DaemonAPI) PostObjectActionProvision(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.PostObjectActionProvisionParams) error {
	if v, err := assertAdmin(ctx, namespace); !v {
		return err
	}
	options := instance.MonitorGlobalExpectOptionsProvisioned{}
	if params.Adopt != nil && *params.Adopt {
		options.Adopt = true
	}
	return a.postObjectAction(ctx, namespace, kind, name, instance.MonitorGlobalExpectProvisioned, func(c *client.T) (*http.Response, error) {
		return c.PostObjectActionProvision(ctx.Request().Context(), namespace, kind, name, &params)
	}, options)
}
