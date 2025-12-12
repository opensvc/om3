package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/dns"
)

// GetDaemonDNSDump returns the DNS zone content.
func (a *DaemonAPI) GetDaemonDNSDump(ctx echo.Context, nodename string) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if nodename == a.localhost {
		return ctx.JSON(http.StatusOK, dns.GetZone())
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.GetDaemonDNSDump(ctx.Request().Context(), nodename)
	})
}
