package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/daemon/rbac"
)

// PostDaemonJoin publishes msgbus.JoinRequest{Node: node} with label node=<apinode>.
// It requires non empty params.Node
func (a *DaemonApi) PostDaemonJoin(ctx echo.Context, params api.PostDaemonJoinParams) error {
	if v, err := assertRole(ctx, rbac.RoleRoot, rbac.RoleJoin); err != nil {
		return err
	} else if !v {
		return nil
	}
	log := LogHandler(ctx, "PostDaemonJoin")
	node := params.Node
	// TODO verify is node value is a valid nodename
	if node == "" {
		log.Warn().Msgf("invalid node value: '%s'", node)
		return JSONProblem(ctx, http.StatusBadRequest, "Invalid parameters", "Missing node param")
	}
	log.Info().Msgf("publish join request for node %s", node)
	a.EventBus.Pub(&msgbus.JoinRequest{Node: node}, labelApi, a.LabelNode)
	return ctx.JSON(http.StatusOK, nil)
}
