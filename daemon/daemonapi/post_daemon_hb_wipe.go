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

func (a *DaemonAPI) PostDaemonHeartbeatWipe(ctx echo.Context, nodename api.InPathNodeName, name api.InPathHeartbeatName) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)

	if nodename == a.localhost || nodename == "localhost" {
		return localPostDaemonHeartbeatWipe(ctx, name)
	}
	return a.proxy(ctx, nodename, func(t *client.T) (*http.Response, error) {
		return t.PostDaemonHeartbeatWipe(ctx.Request().Context(), nodename, name)
	})
}

func localPostDaemonHeartbeatWipe(ctx echo.Context, name api.InPathHeartbeatName) error {
	log := LogHandler(ctx, "postDaemonHeartbeatWipe")
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
		log.Debugf("heartbeat %s is not a disk", name)
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "heartbeat %s is not a disk", name)
	}

	path := config.GetString(key.New(section, "dev"))
	if path == "" {
		log.Warnf("Path %s: %v", path, err)
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "heartbeat %s has no dev configured", name)
	}

	hasSignature, err := sign.EnsureSignature(path)
	if err != nil {
		log.Warnf("EnsureSignature %s: %v", path, err)
		return JSONProblemf(ctx, http.StatusInternalServerError, "EnsureSignature", "%s", err)
	}

	if !hasSignature {
		log.Infof("heartbeat %s on %s has no signature, nothing to wipe", name, path)
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "heartbeat %s on %s has no signature, nothing to wipe", name, path)
	}

	log.Infof("Wipe heartbeat %s on %s", name, path)
	err = sign.RemoveHeaderFromDisk(path)
	if err != nil {
		log.Warnf("RemoveHeaderFromDisk %s: %v", path, err)
		return JSONProblemf(ctx, http.StatusInternalServerError, "RemoveHeaderFromDisk", "%s", err)
	}

	return JSONProblemf(ctx, http.StatusOK, "heartbeat wipe", "wipe heartbeat %s on %s", name, path)
}
