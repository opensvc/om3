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
		log.Warnf("NewCluster: %v", err)
		return JSONProblemf(ctx, http.StatusInternalServerError, "NewCluster", "%s", err)
	}
	config := (i.(configProvider)).Config()
	section := "hb#" + string(name)

	hbType := config.GetString(key.New(section, "type"))
	if hbType != "disk" {
		log.Tracef("heartbeat %s is not a disk", name)
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "heartbeat %s is not a disk", name)
	}

	path := config.GetString(key.New(section, "dev"))
	if path == "" {
		log.Warnf("Path %s: %v", path, err)
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "heartbeat %s has no dev configured", name)
	}

	log.Infof("Sign heartbeat %s on %s", name, path)
	err = sign.CreateAndFillDisk(path)
	if err != nil {
		log.Warnf("CreateAndFillDisk %s: %v", path, err)
		return JSONProblemf(ctx, http.StatusInternalServerError, "CreateAndFillDisk", "%s", err)
	}

	return JSONProblemf(ctx, http.StatusOK, "heartbeat sign", "sign heartbeat %s on %s", name, path)
}
