package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/key"
	"github.com/opensvc/om3/util/sign"
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
	var i any
	i, err := object.NewCluster(object.WithVolatile(true))
	if err != nil {
		log.Warnf("new cluster object failed: %v", err)
		return JSONProblemf(ctx, http.StatusInternalServerError, "new cluster object failed", "%s", err)
	}
	config := (i.(configProvider)).Config()
	section := "hb#" + string(name)

	hbType := config.GetString(key.New(section, "type"))
	if hbType != "disk" {
		log.Tracef("sign heartbeat disk refused: unexpected hb#%s.type %s", name, hbType)
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "sign heartbeat disk refused: unexpected hb#%s.type %s", name, hbType)
	}

	devPath := config.GetString(key.New(section, "dev"))
	if devPath == "" {
		log.Warnf("sign heartbeat disk refused: unexpected empty hb#%s.dev", name)
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "sign heartbeat disk refused: unexpected empty hb#%s.dev", name)
	}

	log.Infof("sign heartbeat disk %s dev %s", name, devPath)
	err = sign.CreateAndFillDisk(devPath)
	if err != nil {
		log.Warnf("sign heartbeat disk %s dev %s: %s", name, devPath, err)
		return JSONProblemf(ctx, http.StatusInternalServerError, "Heartbeat disk sign error", "sign heartbeat disk %s dev %s: %s", name, devPath, err)
	}

	return JSONProblemf(ctx, http.StatusOK, "Heartbeat disk signed", "sign heartbeat %s on %s", name, devPath)
}
