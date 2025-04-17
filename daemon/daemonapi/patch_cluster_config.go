package daemonapi

import (
	"github.com/labstack/echo/v4"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonAPI) PatchClusterConfig(ctx echo.Context, params api.PatchClusterConfigParams) error {
	return a.PatchObjectConfig(ctx, "root", naming.KindCcfg, "cluster", api.PatchObjectConfigParams{
		Delete: params.Delete,
		Unset:  params.Unset,
		Set:    params.Set,
	})
}
