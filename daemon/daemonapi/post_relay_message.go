package daemonapi

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/rbac"
	"github.com/opensvc/om3/daemon/relay"
)

func (a *DaemonAPI) PostRelayMessage(ctx echo.Context) error {
	var (
		payload api.PostRelayMessage
		value   api.RelayMessage
	)
	log := LogHandler(ctx, "PostRelayMessage")
	log.Tracef("starting")

	if v, err := assertGrant(ctx, rbac.GrantHeartbeat); !v {
		return err
	}
	username := userFromContext(ctx).GetUserName()

	if err := ctx.Bind(&payload); err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "%s", err)
	}

	value.ClusterName = payload.ClusterName
	value.ClusterID = payload.ClusterID
	value.Nodename = payload.Nodename
	value.Msg = payload.Msg
	value.UpdatedAt = time.Now()
	value.NodeAddr = ctx.Request().RemoteAddr
	value.Username = username

	relay.Map.Store(username, payload.ClusterID, payload.Nodename, value)
	log.Tracef("stored %s %s", payload.ClusterID, payload.Nodename)
	return JSONProblemf(ctx, http.StatusOK, "stored", "at %s from %s", value.UpdatedAt, value.NodeAddr)
}
