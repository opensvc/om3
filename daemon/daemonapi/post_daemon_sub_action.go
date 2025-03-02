package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/clusternode"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/pubsub"
)

func (a *DaemonAPI) PostDaemonHeartbeatAction(ctx echo.Context, nodename api.InPathNodeName, action api.InPathDaemonSubAction) error {
	return a.postDaemonSubsystemAction(ctx, "heartbeat", nodename, action)
}

func (a *DaemonAPI) PostDaemonListenerAction(ctx echo.Context, nodename api.InPathNodeName, action api.InPathDaemonSubAction) error {
	return a.postDaemonSubsystemAction(ctx, "listener", nodename, action)
}

func (a *DaemonAPI) postDaemonSubsystemAction(ctx echo.Context, sub string, nodename api.InPathNodeName, action api.InPathDaemonSubAction) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}

	payload := api.DaemonSubNameBody{}
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
	poster, err := c.NewPostDaemonSubFunc(sub)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Unhandled daemon sub", "get post daemon sub: %s", err)
	}
	resp, err := poster(ctx.Request().Context(), nodename, action, payload)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
	}
	defer func() { _ = resp.Body.Close() }()
	return ctx.Stream(resp.StatusCode, resp.Header.Get("Content-Type"), resp.Body)
}

func (a *DaemonAPI) localPostDaemonSubAction(ctx echo.Context, action api.InPathDaemonSubAction, payload api.DaemonSubNameBody) error {
	log := LogHandler(ctx, "PostDaemonSubAction")
	log.Debugf("starting")

	switch action {
	case "restart":
	case "start":
	case "stop":
	default:
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "unexpected action: %s", action)
	}
	if len(payload.Name) == 0 {
		return JSONProblemf(ctx, http.StatusOK, "Daemon sub component not found", "Daemon sub component list is empty: skip %s", action)
	}
	log.Infof("asking to %s sub components: %s", action, payload.Name)
	for _, name := range payload.Name {
		log.Infof("ask to %s sub component: %s", action, name)
		a.Publisher.Pub(&msgbus.DaemonCtl{Component: name, Action: string(action)}, pubsub.Label{"id", name}, labelOriginAPI)
	}
	return JSONProblemf(ctx, http.StatusOK, "daemon routines action queued", "%s %s", action, payload.Name)
}
