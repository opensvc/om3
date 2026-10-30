package daemonapi

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/console"
	"github.com/opensvc/om3/v3/daemon/daemonauth"
)

// PostNodeConsole issues the ticket a session of a shell of the node is opened
// with. A hypervisor is root on its guests, and asks it through vsock: no
// other client may, so a root grant does not become a shell over the network.
func (a *DaemonAPI) PostNodeConsole(ctx echo.Context, nodename string) error {
	if v, err := assertStrategy(ctx, daemonauth.StrategyVsock); !v {
		return err
	}
	if nodename = a.parseNodename(nodename); nodename != a.localhost {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", fmt.Sprintf("%s is not this node", nodename))
	}
	return a.issueConsoleTicket(ctx, console.Target{Node: a.localhost, Kind: console.KindShell})
}
