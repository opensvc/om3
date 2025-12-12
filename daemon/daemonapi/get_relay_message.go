package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/rbac"
	"github.com/opensvc/om3/v3/daemon/relay"
)

func (a *DaemonAPI) GetRelayMessage(ctx echo.Context, params api.GetRelayMessageParams) error {
	if v, err := assertGrant(ctx, rbac.GrantHeartbeat, rbac.GrantRoot); !v {
		return err
	}
	var username string
	if grantsFromContext(ctx).HasGrant(rbac.GrantRoot) && params.Username != nil {
		username = *params.Username
	} else {
		username = userFromContext(ctx).GetUserName()
	}
	if slot, ok := relay.Map.Load(username, params.ClusterID, params.Nodename); !ok {
		return JSONProblem(ctx, http.StatusNotFound, "Not found", "")
	} else {
		message := slot.Value.(api.RelayMessage)
		message.Relay = a.localhost
		return ctx.JSON(http.StatusOK, message)
	}
}
