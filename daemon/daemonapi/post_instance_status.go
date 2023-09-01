package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/pubsub"
)

func (a *DaemonApi) PostInstanceStatus(ctx echo.Context) error {
	var (
		err     error
		p       path.T
		payload api.PostInstanceStatus
	)
	log := LogHandler(ctx, "PostInstanceStatus")
	log.Debug().Msgf("starting")
	if err := ctx.Bind(&payload); err != nil {
		log.Warn().Err(err).Msgf("decode body")
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "%s", err)
	}
	p, err = path.Parse(payload.Path)
	if err != nil {
		log.Warn().Err(err).Msgf("can't parse path: %s", payload.Path)
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "Error parsing path '%s': %s", payload.Path, err)
	}
	localhost := hostname.Hostname()
	a.EventBus.Pub(&msgbus.InstanceStatusPost{Path: p, Node: localhost, Value: payload.Status},
		pubsub.Label{"path", payload.Path},
		pubsub.Label{"node", localhost},
	)
	return ctx.JSON(http.StatusOK, nil)
}
