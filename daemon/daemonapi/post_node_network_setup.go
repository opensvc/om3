package daemonapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/msgbus"
)

// networkSetupTimeout bounds the wait of a network setup request: a setup
// takes seconds, and a hung one, as one waiting for a name of a peer to
// resolve, is warned about by the node monitor.
var networkSetupTimeout = 2 * time.Minute

// PostNodeNetworkSetup sets up the networks on the node, by its node
// monitor, which runs one setup at a time.
func (a *DaemonAPI) PostNodeNetworkSetup(eCtx echo.Context, nodename api.InPathNodeName, params api.PostNodeNetworkSetupParams) error {
	if v, err := assertRoot(eCtx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if nodename != a.localhost {
		return a.proxy(eCtx, nodename, func(c *client.T) (*http.Response, error) {
			return c.PostNodeNetworkSetup(eCtx.Request().Context(), nodename, &params)
		})
	}
	var names []string
	if params.Name != nil {
		names = *params.Name
	}
	ctx, cancel := context.WithTimeout(eCtx.Request().Context(), networkSetupTimeout)
	defer cancel()
	msg, errReceiver := msgbus.NewNetworkSetupRequestWithErr(ctx, a.localhost, names)
	a.Bus.Pub(msg, a.LabelLocalhost, labelOriginAPI)
	switch err := errReceiver.Receive(); {
	case err == nil:
		return eCtx.NoContent(http.StatusNoContent)
	case errors.Is(err, context.DeadlineExceeded):
		return JSONProblemf(eCtx, http.StatusRequestTimeout, "network setup", "the setup did not end in %s: the node monitor goes on with it", networkSetupTimeout)
	case errors.Is(err, context.Canceled):
		return JSONProblemf(eCtx, http.StatusRequestTimeout, "network setup", "client context canceled")
	default:
		return JSONProblemf(eCtx, http.StatusInternalServerError, "network setup", "%s", err)
	}
}
