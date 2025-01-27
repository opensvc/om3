package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/daemon/rbac"
)

// PostDaemonLeave publishes msgbus.LeaveRequest{Node: node} with label node=<apinode>.
// It requires non empty params.Node
func (a *DaemonAPI) PostDaemonLeave(ctx echo.Context, params api.PostDaemonLeaveParams) error {
	if v, err := assertRole(ctx, rbac.RoleRoot, rbac.RoleLeave); err != nil {
		return err
	} else if !v {
		return nil
	}
	log := LogHandler(ctx, "PostDaemonLeave")
	node := params.Node
	// TODO verify is node value is a valid nodename
	if node == "" {
		log.Warnf("invalid node value: '%s'", node)
		return JSONProblem(ctx, http.StatusBadRequest, "Invalid parameters", "Missing node param")
	} else if node == a.localhost {
		log.Infof("removal of localhost from cluster node is forbidden")
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "node '%s' is localhost", node)
	}
	log.Infof("publish leave request for node %s", node)
	a.Publisher.Pub(&msgbus.LeaveRequest{Node: node}, a.LabelLocalhost, labelOriginAPI)
	return ctx.JSON(http.StatusOK, nil)
}
