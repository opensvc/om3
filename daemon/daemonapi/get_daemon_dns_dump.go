package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/daemon/dns"
)

// GetDaemonDNSDump returns the DNS zone content.
func (a *DaemonApi) GetDaemonDNSDump(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, dns.GetZone())
}
