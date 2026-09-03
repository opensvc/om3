package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/sign"
)

func (a *DaemonAPI) PostDaemonHeartbeatSign(ctx echo.Context, nodename api.InPathNodeName, name api.InPathHeartbeatName) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)

	if nodename == a.localhost || nodename == "localhost" {
		return localPostDaemonHeartbeatSign(ctx, name)
	}
	return a.proxy(ctx, nodename, func(t *client.T) (*http.Response, error) {
		return t.PostDaemonHeartbeatSign(ctx.Request().Context(), nodename, name)
	})
}

func localPostDaemonHeartbeatSign(ctx echo.Context, name api.InPathHeartbeatName) error {
	log := LogHandler(ctx, "postDaemonHeartbeatSign")
	section, err := heartbeatName(name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "%s", err)
	}
	var i any
	i, err = object.NewCluster(object.WithVolatile(true))
	if err != nil {
		log.Warnf("new cluster object failed: %v", err)
		return JSONProblemf(ctx, http.StatusInternalServerError, "new cluster object failed", "%s", err)
	}
	config := (i.(configProvider)).Config()

	hbType := config.GetString(key.New(section, "type"))
	if hbType != "disk" {
		log.Tracef("sign heartbeat disk refused: unexpected %s.type %s", section, hbType)
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "sign heartbeat disk refused: unexpected %s.type %s", section, hbType)
	}

	devPath := config.GetString(key.New(section, "dev"))
	if devPath == "" {
		log.Warnf("sign heartbeat disk refused: unexpected empty %s.dev", section)
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "sign heartbeat disk refused: unexpected empty %s.dev", section)
	}

	log.Infof("sign heartbeat disk %s dev %s", section, devPath)
	err = sign.CreateAndFillDisk(devPath)
	if err != nil {
		log.Warnf("sign heartbeat disk %s dev %s: %s", section, devPath, err)
		return JSONProblemf(ctx, http.StatusInternalServerError, "Heartbeat disk sign error", "sign heartbeat disk %s dev %s: %s", section, devPath, err)
	}

	return JSONProblemf(ctx, http.StatusOK, "Heartbeat disk signed", "sign heartbeat %s on %s", section, devPath)
}
