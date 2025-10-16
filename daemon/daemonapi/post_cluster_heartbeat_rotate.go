package daemonapi

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/msgbus"
)

func (a *DaemonAPI) PostClusterHeartbeatRotate(ctx echo.Context) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}

	leader, err := getLeaderNode()
	if err != nil {
		return JSONProblemf(ctx, http.StatusConflict, "cluster heartbeat rotate refused", "can't get leader node: %s", err)
	}

	if leader == a.localhost {
		return a.postLocalClusterHeartbeatRotate(ctx)
	} else {
		return a.proxy(ctx, leader, func(c *client.T) (*http.Response, error) {
			return c.PostClusterHeartbeatRotate(ctx.Request().Context())
		})
	}
}

func (a *DaemonAPI) postLocalClusterHeartbeatRotate(ctx echo.Context) error {
	log := LogHandler(ctx, "postLocalClusterHeartbeatRotate")
	log.Infof("publish heartbeat rotate request")
	id := ctx.Get("uuid").(uuid.UUID)
	a.Publisher.Pub(&msgbus.HeartbeatRotateRequest{ID: id}, a.LabelLocalhost, labelOriginAPI)
	return ctx.JSON(http.StatusOK, api.HeartbeatRotateResponse{ID: id})
}
