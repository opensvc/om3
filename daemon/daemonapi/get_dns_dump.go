package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/daemon/dns"
)

// GetDNSDump returns the DNS zone content.
func (a *DaemonAPI) GetDNSDump(ctx echo.Context) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	return ctx.JSON(http.StatusOK, dns.GetZone())
}
