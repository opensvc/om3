package daemonapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/hb/hbdisk"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/sign"
)

func (a *DaemonAPI) PostDaemonHeartbeatWipe(ctx echo.Context, nodename api.InPathNodeName, name api.InPathHeartbeatName, params api.PostDaemonHeartbeatWipeParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)

	if nodename == a.localhost || nodename == "localhost" {
		return localPostDaemonHeartbeatWipe(ctx, name, params)
	}
	return a.proxy(ctx, nodename, func(t *client.T) (*http.Response, error) {
		return t.PostDaemonHeartbeatWipe(ctx.Request().Context(), nodename, name, &params)
	})
}

func localPostDaemonHeartbeatWipe(ctx echo.Context, name api.InPathHeartbeatName, params api.PostDaemonHeartbeatWipeParams) error {
	log := LogHandler(ctx, "postDaemonHeartbeatWipe")
	section, err := heartbeatName(name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "%s", err)
	}
	var i any
	i, err = object.NewCluster(object.WithVolatile(true))
	if err != nil {
		log.Warnf("new cluster object failed: %v", err)
		return JSONProblemf(ctx, http.StatusInternalServerError, "NewCluster", "new cluster object failed: %v", err)
	}
	config := (i.(configProvider)).Config()

	hbType := config.GetString(key.New(section, "type"))
	if hbType != "disk" {
		log.Tracef("refuse to wipe heartbeat disk: unexpected %s.type %s", section, hbType)
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "refuse to wipe heartbeat disk: unexpected %s.type %s", section, hbType)
	}

	devPath := config.GetString(key.New(section, "dev"))
	if devPath == "" {
		log.Warnf("refuse to wipe heartbeat disk: unexpected empty %s.dev", section)
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "refuse to wipe heartbeat disk: unexpected empty %s.dev", section)
	}

	hasSignature, err := sign.EnsureSignature(devPath)
	if err != nil {
		log.Warnf("ensure signature failed on %s: %w", devPath, err)
		return JSONProblemf(ctx, http.StatusInternalServerError, "EnsureSignature", "ensure signature failed on %s: %s", devPath, err)
	}

	if !hasSignature {
		log.Infof("heartbeat %s dev %s has no signature, nothing to wipe", section, devPath)
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "heartbeat %s dev %s has no signature, nothing to wipe", section, devPath)
	}

	if params.Force == nil || !*params.Force {
		window := foreignWriteWindow(config, section)
		beating, err := hbdisk.BeatingForeignNodes(ctx.Request().Context(), devPath, config.GetInt(key.New(section, "max_slots")), heartbeatNodes(config, section), window)
		if err != nil {
			log.Warnf("look for the nodes of other clusters beating on %s: %s", devPath, err)
			return JSONProblemf(ctx, http.StatusInternalServerError, "BeatingForeignNodes", "look for the nodes of other clusters beating on %s: %s", devPath, err)
		}
		if len(beating) > 0 {
			log.Infof("refuse to wipe heartbeat %s dev %s: nodes %s beat on it", section, devPath, strings.Join(beating, " "))
			return JSONProblemf(ctx, http.StatusConflict, "Heartbeat disk in use",
				"refuse to wipe heartbeat %s dev %s: nodes %s, not of this heartbeat, wrote to it within %s, and their heartbeats would lose its signature too. Stop them, or wipe with --force",
				section, devPath, strings.Join(beating, " "), window)
		}
	}

	log.Infof("wipe heartbeat %s dev %s", section, devPath)
	err = sign.RemoveHeaderFromDisk(devPath)
	if err != nil {
		log.Warnf("wipe heartbeat disk %s dev %s failed: remove header: %s", section, devPath, err)
		return JSONProblemf(ctx, http.StatusInternalServerError, "RemoveHeaderFromDisk", "wipe heartbeat disk %s dev %s failed: remove header: %s", section, devPath, err)
	}

	return JSONProblemf(ctx, http.StatusOK, "heartbeat disk wiped", "wipe heartbeat %s on %s", section, devPath)
}

// maxForeignWriteWindow bounds how long a wipe watches the disk for the
// writes of other clusters, under the client timeout.
const maxForeignWriteWindow = 20 * time.Second

// foreignWriteWindow returns how long a wipe watches the disk of the
// heartbeat for the writes of other clusters: the timeout of the heartbeat,
// which a live one writes within, other clusters sharing the disk being
// configured alike.
func foreignWriteWindow(config *xconfig.T, section string) time.Duration {
	timeout, interval := hbdisk.DefaultTimeout, hbdisk.DefaultInterval
	if d := config.GetDuration(key.New(section, "timeout")); d != nil {
		timeout = *d
	}
	if d := config.GetDuration(key.New(section, "interval")); d != nil {
		interval = *d
	}
	return min(max(timeout, hbdisk.MinTimeout(interval)), maxForeignWriteWindow)
}

// heartbeatNodes returns the nodes of the heartbeat: its own nodes, or the
// cluster nodes.
func heartbeatNodes(config *xconfig.T, section string) []string {
	if nodes := config.GetStrings(key.New(section, "nodes")); len(nodes) > 0 {
		return nodes
	}
	return config.GetStrings(key.New("cluster", "nodes"))
}
