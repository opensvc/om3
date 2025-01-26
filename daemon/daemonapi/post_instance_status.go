package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/daemonauth"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/pubsub"
)

func (a *DaemonAPI) PostInstanceStatus(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
	if ok, err := assertStrategy(ctx, daemonauth.StrategyUX); !ok {
		return err
	}
	var payload api.InstanceStatus
	log := LogHandler(ctx, "PostInstanceStatus")
	log.Debugf("starting")
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		log.Warnf("can't make path: %s", err)
		_ = JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "Error making path: %s", err)
		return err
	}
	if err := ctx.Bind(&payload); err != nil {
		log.Warnf("decode body: %s", err)
		_ = JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "%s", err)
		return err
	}
	a.Pub.Pub(&msgbus.InstanceStatusPost{Path: p, Node: a.localhost, Value: payload},
		pubsub.Label{"namespace", p.Namespace},
		pubsub.Label{"path", p.String()},
		a.LabelLocalhost,
		labelOriginAPI,
	)
	return ctx.JSON(http.StatusOK, nil)
}
