package daemonapi

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/network"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/api"
)

// GetNetworks returns network status list.
func (a *DaemonApi) GetNetwork(ctx echo.Context, params api.GetNetworkParams) error {
	n, err := object.NewNode(object.WithVolatile(true))
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Failed to allocate a new object.Node", fmt.Sprint(err))
	}
	cips := GetClusterIps()
	var l network.StatusList
	if params.Name != nil {
		l = network.ShowNetworksByName(n, *params.Name, cips)
	} else {
		l = network.ShowNetworks(n, cips)
	}
	return ctx.JSON(http.StatusOK, l)
}
