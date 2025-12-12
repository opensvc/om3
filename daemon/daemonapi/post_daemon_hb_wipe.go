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
		log.Warnf("new cluster object failed: %v", err)
		return JSONProblemf(ctx, http.StatusInternalServerError, "NewCluster", "new cluster object failed: %v", err)
	}
	config := (i.(configProvider)).Config()
	section := "hb#" + string(name)

	hbType := config.GetString(key.New(section, "type"))
	if hbType != "disk" {
		log.Tracef("refuse to wipe heartbeat disk: unexpected hb#%s.type %s", name, hbType)
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "refuse to wipe heartbeat disk: unexpected hb#%s.type %s", name, hbType)
	}

	devPath := config.GetString(key.New(section, "dev"))
	if devPath == "" {
		log.Warnf("refuse to wipe heartbeat disk: unexpected empty hb#%s.dev", name)
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "refuse to wipe heartbeat disk: unexpected empty hb#%s.dev", name)
	}

	hasSignature, err := sign.EnsureSignature(devPath)
	if err != nil {
		log.Warnf("ensure signature failed on %s: %w", devPath, err)
		return JSONProblemf(ctx, http.StatusInternalServerError, "EnsureSignature", "ensure signature failed on %s: %s", devPath, err)
	}

	if !hasSignature {
		log.Infof("heartbeat %s dev %s has no signature, nothing to wipe", name, devPath)
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "heartbeat %s dev %s has no signature, nothing to wipe", name, devPath)
	}

	log.Infof("wipe heartbeat %s dev %s", name, devPath)
	err = sign.RemoveHeaderFromDisk(devPath)
	if err != nil {
		log.Warnf("wipe heartbeat disk %s dev %s failed: remove header: %s", name, devPath, err)
		return JSONProblemf(ctx, http.StatusInternalServerError, "RemoveHeaderFromDisk", "wipe heartbeat disk %s dev %s failed: remove header: %s", name, devPath, err)
	}

	return JSONProblemf(ctx, http.StatusOK, "heartbeat disk wiped", "wipe heartbeat %s on %s", name, devPath)
}
