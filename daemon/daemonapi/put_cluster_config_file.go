package daemonapi

import (
	"io"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/naming"
)

func (a *DaemonAPI) PutClusterConfigFile(ctx echo.Context) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	base, err := readConfigBase(naming.Cluster)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Read config", "%s", err)
	}
	body, err := io.ReadAll(ctx.Request().Body)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Read body", "%s", err)
	}
	return a.writeObjectConfigFile(ctx, naming.Cluster, body, base, nil, false)
}
