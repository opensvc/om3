package daemonapi

import (
	"github.com/labstack/echo/v4"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonAPI) GetClusterConfig(ctx echo.Context, params api.GetClusterConfigParams) error {
	return a.GetObjectConfig(ctx, naming.Cluster.Namespace, naming.Cluster.Kind, naming.Cluster.Name, api.GetObjectConfigParams{
		Evaluate:    params.Evaluate,
		Impersonate: params.Impersonate,
		Kw:          params.Kw,
	})
}
