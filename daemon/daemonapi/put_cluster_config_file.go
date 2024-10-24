package daemonapi

import (
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/naming"
)

func (a *DaemonAPI) PutClusterConfigFile(ctx echo.Context) error {
	return a.writeObjectConfigFile(ctx, naming.Cluster)
}
