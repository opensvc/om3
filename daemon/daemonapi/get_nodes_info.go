package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/nodesinfo"
)

func (a *DaemonAPI) GetNodesInfo(ctx echo.Context) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	log := LogHandler(ctx, "GetNodesInfo")
	log.Tracef("starting")
	// TODO returned value should be cached
	data, err := nodesinfo.Load()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "nodes info cache load", "%s", err)
	}
	return ctx.JSON(http.StatusOK, data)
}
