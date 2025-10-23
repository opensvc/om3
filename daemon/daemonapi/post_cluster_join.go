package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/daemon/rbac"
)

// PostClusterJoin publishes msgbus.JoinRequest{Node: node} with label node=<apinode>.
// It requires non empty params.Node
func (a *DaemonAPI) PostClusterJoin(ctx echo.Context, params api.PostClusterJoinParams) error {
	if v, err := assertRole(ctx, rbac.RoleRoot, rbac.RoleJoin); !v {
		return err
	}
	log := LogHandler(ctx, "PostClusterJoin")
	node := params.Node
	// TODO verify is node value is a valid nodename
	if node == "" {
		log.Warnf("invalid node value: '%s'", node)
		return JSONProblem(ctx, http.StatusBadRequest, "Invalid parameters", "Missing node param")
	}
	log.Infof("publish join request for node %s", node)
	a.Publisher.Pub(&msgbus.JoinRequest{Node: node}, a.LabelLocalhost, labelOriginAPI)
	ctx.Response().Header().Add(api.HeaderLastModified, "")
	return ctx.JSON(http.StatusOK, nil)
}
