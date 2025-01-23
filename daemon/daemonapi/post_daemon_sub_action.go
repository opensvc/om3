package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/pubsub"
)

func (a *DaemonAPI) PostDaemonSubAction(ctx echo.Context) error {
	if _, err := assertRoot(ctx); err != nil {
		return err
	}

	log := LogHandler(ctx, "PostDaemonSubAction")
	log.Debugf("starting")

	var (
		payload api.PostDaemonSubAction
	)
	if err := ctx.Bind(&payload); err != nil {
		log.Warnf("invalid body: %s", err)
		return JSONProblem(ctx, http.StatusBadRequest, "Invalid body", err.Error())
	}
	action := string(payload.Action)
	switch action {
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
		a.EventBus.Pub(&msgbus.DaemonCtl{Component: sub, Action: action}, pubsub.Label{"id", sub}, labelOriginAPI)
	}
	return JSONProblemf(ctx, http.StatusOK, "daemon routines action queued", "%s %s", action, subs)
}
