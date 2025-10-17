package daemonapi

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/msgbus"
)

var (
	// heartbeatRotateLock is a mutex used to enforce rate limiting for cluster
	// heartbeat rotation operations.
	heartbeatRotateLock = sync.Mutex{}

	// heartbeatRotateMinCallInterval defines the minimum time interval required
	// between successive heartbeat rotate calls.
	heartbeatRotateMinCallInterval = 60 * time.Second
)

func (a *DaemonAPI) PostClusterHeartbeatRotate(ctx echo.Context) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}

	leader, err := getLeaderNode()
	if err != nil {
		log := LogHandler(ctx, "PostClusterHeartbeatRotate")
		log.Infof("unable to get leader node: %s", err)
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
	log := LogHandler(ctx, "PostClusterHeartbeatRotate")
	if ok := heartbeatRotateLock.TryLock(); !ok {
		log := LogHandler(ctx, "postLocalClusterHeartbeatRotate")
		err := fmt.Errorf("initiated too early, rate limiting is enforced")
		log.Infof("cluster heartbeat rotate refused: %s", err)
		return JSONProblemf(ctx, http.StatusConflict, "cluster heartbeat rotate refused", "%s", err)
	}
	log.Infof("publish heartbeat rotate request")
	id := ctx.Get("uuid").(uuid.UUID)
	a.Publisher.Pub(&msgbus.HeartbeatRotateRequest{ID: id}, a.LabelLocalhost, labelOriginAPI)
	go func() {
		time.AfterFunc(heartbeatRotateMinCallInterval, func() {
			heartbeatRotateLock.Unlock()
		})
	}()
	return ctx.JSON(http.StatusOK, api.HeartbeatRotateResponse{ID: id})
}
