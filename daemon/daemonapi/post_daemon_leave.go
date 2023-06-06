package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/daemonauth"
	"github.com/opensvc/om3/daemon/msgbus"
)

// PostDaemonLeave publishes msgbus.LeaveRequest{Node: node} with label node=<apinode>.
// It requires non empty params.Node
func (a *DaemonApi) PostDaemonLeave(ctx echo.Context, params api.PostDaemonLeaveParams) error {
	var (
		node string
	)
	log := LogHandler(ctx, "PostDaemonLeave")

	grants := GrantsFromContext(ctx)
	if !grants.HasAnyRole(daemonauth.RoleRoot, daemonauth.RoleLeave) {
		log.Info().Msg("not allowed, need at least 'root' or 'leave' grant")
		return JSONProblemf(ctx, http.StatusForbidden, "Missing grants", "not allowed, need at least 'root' or 'leave' grant, have %s", grants)
	}

	node = params.Node
	// TODO verify is node value is a valid nodename
	if node == "" {
		log.Warn().Msgf("invalid node value: '%s'", node)
		return JSONProblem(ctx, http.StatusBadRequest, "Invalid parameters", "Missing node param")
	}
	log.Info().Msgf("publish leave request for node %s", node)
	a.EventBus.Pub(&msgbus.LeaveRequest{Node: node}, labelApi, labelNode)
	return ctx.JSON(http.StatusOK, nil)
}
