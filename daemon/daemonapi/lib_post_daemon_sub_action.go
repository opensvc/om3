package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/pubsub"
)

func (a *DaemonAPI) postDaemonSubAction(ctx echo.Context, nodename api.InPathNodeName, action, localName string, fn func(c *client.T) (*http.Response, error)) error {
	if len(localName) == 0 {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "sub component localName is empty")
	}
	switch action {
	case "restart":
	case "start":
	case "stop":
	case "log-level-panic":
	case "log-level-fatal":
	case "log-level-error":
	case "log-level-warn":
	case "log-level-info":
	case "log-level-debug":
	case "log-level-trace":
	default:
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "unexpected action: %s", action)
	}
	if nodename == a.localhost || nodename == "localhost" {
		log := LogHandler(ctx, "postDaemonSubAction")
		log.Infof("ask to %s component: %s", action, localName)
		a.Publisher.Pub(&msgbus.DaemonCtl{Component: localName, Action: action}, pubsub.Label{"id", localName}, labelOriginAPI)
		return JSONProblemf(ctx, http.StatusOK, "daemon action queued", "%s %s", action, localName)
	}
	return a.proxy(ctx, nodename, fn)
}
