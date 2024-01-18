package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/node"
)

func (a *DaemonAPI) GetNodesInfo(ctx echo.Context) error {
	log := LogHandler(ctx, "GetNodesInfo")
	log.Debugf("starting")
	// TODO returned value should be cached
	data := node.GetNodesInfo()
	return ctx.JSON(http.StatusOK, data)
}
