package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
)

func (a *DaemonAPI) PostObjectActionStop(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.PostObjectActionStopParams) error {
	if v, err := assertOperator(ctx, namespace); !v {
		return err
	}
	options := instance.MonitorGlobalExpectOptionsStopped{
		InterruptSyncs: params.InterruptSyncs != nil && *params.InterruptSyncs,
	}
	return a.postObjectAction(ctx, namespace, kind, name, instance.MonitorGlobalExpectStopped, func(c *client.T) (*http.Response, error) {
		return c.PostObjectActionStop(ctx.Request().Context(), namespace, kind, name, &params)
	}, options)
}
