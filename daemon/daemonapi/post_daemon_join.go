package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/daemonauth"
	"github.com/opensvc/om3/daemon/msgbus"
)

// PostDaemonJoin publishes msgbus.JoinRequest{Node: node} with label node=<apinode>.
// It requires non empty params.Node
func (a *DaemonApi) PostDaemonJoin(ctx echo.Context, params api.PostDaemonJoinParams) error {
	var (
		node string
	)
	log := LogHandler(ctx, "PostDaemonJoin")
	grants := GrantsFromContext(ctx)
	if !grants.HasAnyRole(daemonauth.RoleRoot, daemonauth.RoleJoin) {
		log.Info().Msg("not allowed, need at least 'root' or 'join' grant")
		return JSONProblemf(ctx, http.StatusForbidden, "Missing grants", "not allowed, need at least 'root' or 'join' grant, have %s", grants)
	}

	node = params.Node
	// TODO verify is node value is a valid nodename
	if node == "" {
		log.Warn().Msgf("invalid node value: '%s'", node)
		return JSONProblem(ctx, http.StatusBadRequest, "Invalid parameters", "Missing node param")
	}
	log.Info().Msgf("publish join request for node %s", node)
	a.EventBus.Pub(&msgbus.JoinRequest{Node: node}, labelApi, labelNode)
	return ctx.JSON(http.StatusOK, nil)
}
