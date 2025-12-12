package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/daemon/rbac"
)

// PostClusterJoin publishes msgbus.JoinRequest{Node: node} with label node=<apinode>.
// It requires non empty params.Node
func (a *DaemonAPI) PostClusterJoin(ctx echo.Context, params api.PostClusterJoinParams) error {
	if v, err := assertRole(ctx, rbac.RoleRoot, rbac.RoleJoin); !v {
		return err
	}
	log := LogHandler(ctx, "PostClusterJoin")
	candidate := params.Node
	// TODO verify is node value is a valid nodename
	if candidate == "" {
		log.Warnf("invalid node value: '%s'", candidate)
		return JSONProblem(ctx, http.StatusBadRequest, "Invalid parameters", "Missing node param")
	}
	log.Infof("publish join request for node %s", candidate)
	a.Publisher.Pub(&msgbus.JoinRequest{CandidateNode: candidate}, a.LabelLocalhost, labelOriginAPI)
	ctx.Response().Header().Add(api.HeaderServedBy, a.localhost)
	return ctx.JSON(http.StatusOK, nil)
}
