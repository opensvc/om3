package daemonapi

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/pubsub"
)

// postDaemonSubAction publishes the action for each of the components the
// request names.
//
// A name can designate more than one component: a heartbeat is both of its
// streams, which run and stop on their own but are one thing to the caller
// naming the heartbeat.
func (a *DaemonAPI) postDaemonSubAction(ctx echo.Context, nodename api.InPathNodeName, action string, localNames []string, fn func(c *client.T) (*http.Response, error)) error {
	if len(localNames) == 0 {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "sub component localName is empty")
	}
	for _, localName := range localNames {
		if len(localName) == 0 {
			return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "sub component localName is empty")
		}
	}
	switch action {
	case "restart":
	case "start":
	case "stop":
	default:
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "unexpected action: %s", action)
	}
	if nodename == a.localhost || nodename == "localhost" {
		log := LogHandler(ctx, "postDaemonSubAction")
		for _, localName := range localNames {
			log.Infof("ask to %s component: %s", action, localName)
			a.Bus.Pub(&msgbus.DaemonCtl{Component: localName, Action: action}, pubsub.Label{"id", localName}, labelOriginAPI)
		}
		return JSONProblemf(ctx, http.StatusOK, "daemon action queued", "%s %s", action, strings.Join(localNames, ", "))
	}
	return a.proxy(ctx, nodename, fn)
}
