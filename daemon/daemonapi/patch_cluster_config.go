package daemonapi

import (
	"github.com/labstack/echo/v4"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
)

func (a *DaemonAPI) PatchClusterConfig(ctx echo.Context, params api.PatchClusterConfigParams) error {
	return a.PatchObjectConfig(ctx, naming.Cluster.Namespace, naming.Cluster.Kind, naming.Cluster.Name, api.PatchObjectConfigParams{
		Delete: params.Delete,
		Unset:  params.Unset,
		Set:    params.Set,
	})
}
