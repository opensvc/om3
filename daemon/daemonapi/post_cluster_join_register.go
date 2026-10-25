package daemonapi

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/core/collector"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/rbac"
)

// PostClusterJoinRegister registers on the collector a node joining the
// cluster, on its behalf.
//
// The join moves the joining node configuration aside, its registration id
// included, so it can't register itself before its daemon starts. This node,
// authenticated on the collector by its own registration id, registers it
// with the registration id it held before the join, and returns the new one
// for the joining node to store. The collector moves the joining node to the
// app and the cluster of this node.
func (a *DaemonAPI) PostClusterJoinRegister(ctx echo.Context) error {
	if v, err := assertRole(ctx, rbac.RoleRoot, rbac.RoleJoin); !v {
		return err
	}
	log := LogHandler(ctx, "PostClusterJoinRegister")

	var payload api.ClusterJoinRegisterBody
	if err := ctx.Bind(&payload); err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "%s", err)
	}
	if payload.Node == "" {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "missing 'node' field")
	}
	if payload.Uuid == "" {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "missing 'uuid' field")
	}
	// A join token holder registers a node of the cluster only: the join
	// has added it to the cluster nodes before.
	if !clusternode.Has(payload.Node) {
		return JSONProblemf(ctx, http.StatusConflict, "Not a cluster node", "%s is not a cluster node", payload.Node)
	}

	n, err := object.NewNode()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New node", "%s", err)
	}
	nodeUUID, err := n.RegisterJoiningNode(ctx.Request().Context(), payload.Node, payload.Uuid)
	switch {
	case errors.Is(err, collector.ErrConfig), errors.Is(err, collector.ErrUnregistered):
		log.Infof("can't register the joining node %s on the collector: %s", payload.Node, err)
		return JSONProblemf(ctx, http.StatusConflict, "Collector", "%s", err)
	case err != nil:
		log.Warnf("register the joining node %s on the collector: %s", payload.Node, err)
		return JSONProblemf(ctx, http.StatusBadGateway, "Collector", "%s", err)
	}
	log.Infof("registered the joining node %s on the collector", payload.Node)
	return ctx.JSON(http.StatusOK, api.ClusterJoinRegistered{Uuid: nodeUUID})
}
