package daemonapi

import (
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/naming"
)

func (a *DaemonAPI) PutClusterConfigFile(ctx echo.Context) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	return a.writeObjectConfigFile(ctx, naming.Cluster)
}
