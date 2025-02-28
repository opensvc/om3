package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/clusternode"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/pubsub"
)

func (a *DaemonAPI) PostDaemonComponentAction(ctx echo.Context, nodename api.InPathNodeName, action api.PostDaemonComponentActionParamsAction) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}

	payload := api.PostDaemonComponentActionBody{}
	if err := ctx.Bind(&payload); err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "%s", err)
	}

	if nodename == a.localhost {
		return a.localPostDaemonSubAction(ctx, action, payload)
	} else if !clusternode.Has(nodename) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid nodename", "field 'nodename' with value '%s' is not a cluster node", nodename)
	}
	c, err := a.newProxyClient(ctx, nodename)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
	}
	resp, err := c.PostDaemonComponentActionWithResponse(ctx.Request().Context(), nodename, action, payload)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
	} else if len(resp.Body) > 0 {
		return ctx.JSONBlob(resp.StatusCode(), resp.Body)
	}
	return nil
}

func (a *DaemonAPI) localPostDaemonSubAction(ctx echo.Context, action api.PostDaemonComponentActionParamsAction, payload api.PostDaemonComponentActionBody) error {
	log := LogHandler(ctx, "PostDaemonSubAction")
	log.Debugf("starting")

	switch action {
	case "restart":
	case "start":
	case "stop":
	default:
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "unexpected action: %s", action)
	}
	var subs []string
	for _, sub := range payload.Subs {
		subs = append(subs, sub)
	}
	if len(subs) == 0 {
		return JSONProblemf(ctx, http.StatusOK, "Daemon routine not found", "No daemon routine to %s", action)
	}
	log.Infof("asking to %s sub components: %s", action, subs)
	for _, sub := range payload.Subs {
		log.Infof("ask to %s sub component: %s", action, sub)
		a.Publisher.Pub(&msgbus.DaemonCtl{Component: sub, Action: string(action)}, pubsub.Label{"id", sub}, labelOriginAPI)
	}
	return JSONProblemf(ctx, http.StatusOK, "daemon routines action queued", "%s %s", action, subs)
}
