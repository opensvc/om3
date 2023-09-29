package daemonapi

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/pubsub"
)

func (a *DaemonApi) PostInstanceStatus(ctx echo.Context) error {
	var (
		err     error
		p       naming.Path
		payload api.InstanceStatusItem
	)
	log := LogHandler(ctx, "PostInstanceStatus")
	log.Debug().Msgf("starting")
	if err := ctx.Bind(&payload); err != nil {
		log.Warn().Err(err).Msgf("decode body")
		_ = JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "%s", err)
		return err
	}
	p, err = naming.ParsePath(payload.Meta.Object)
	if err != nil {
		log.Warn().Err(err).Msgf("can't parse path: %s", payload.Meta.Object)
		_ = JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "Error parsing path '%s': %s", payload.Meta.Object, err)
		return err
	}
	localhost := hostname.Hostname()
	if payload.Meta.Node != localhost {
		err := fmt.Errorf("meta node is %s: expecting %s", payload.Meta.Node, localhost)
		_ = JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "%s", err)
		return err
	}
	a.EventBus.Pub(&msgbus.InstanceStatusPost{Path: p, Node: payload.Meta.Node, Value: payload.Data},
		pubsub.Label{"path", payload.Meta.Object},
		pubsub.Label{"node", payload.Meta.Node},
	)
	return ctx.JSON(http.StatusOK, nil)
}
