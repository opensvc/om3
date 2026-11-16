package daemonapi

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/pubsub"
)

func (a *DaemonAPI) GetObjectConfigFile(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.GetObjectConfigFileParams) error {
	logName := "GetObjectConfigFile"
	log := LogHandler(ctx, logName)
	log.Tracef("%s: starting", logName)

	objPath, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		log.Warnf("%s: %s", logName, err)
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "invalid path: %s", err)
	}
	redact, ok, err := configReadAccess(ctx, objPath)
	if !ok {
		return err
	}
	if params.RedactSecrets != nil && *params.RedactSecrets {
		redact = true
	}
	log = naming.LogWithPath(log, objPath)

	// Before the proxy to a peer too: the local config record may be gone
	// already as the local monitor deletes the object.
	if a.configBeingDeleted(objPath) {
		log.Infof("%s: %s is being deleted: config file not served", logName, objPath)
		return JSONProblemf(ctx, http.StatusNotFound, "Not found", "%s is being deleted", objPath)
	}
	if instMon := instance.ConfigData.GetByPathAndNode(objPath, a.localhost); instMon != nil {
		filename := objPath.ConfigFile()
		mtime := file.ModTime(filename)
		if mtime.IsZero() {
			log.Infof("%s: config file not found: %s", logName, filename)
			return JSONProblemf(ctx, http.StatusNotFound, "Not found", "config file not found: %s", filename)
		}

		ctx.Response().Header().Add(api.HeaderLastModified, mtime.Format(time.RFC3339Nano))
		requester := userFromContext(ctx).Username
		log.Infof("serve config file %s to %s", objPath, requester)
		a.announceConfigServed(objPath, requester)

		return serveConfigFile(ctx, objPath, filename, redact)
	}
	for nodename := range instance.ConfigData.GetByPath(objPath) {
		return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.GetObjectConfigFile(ctx.Request().Context(), namespace, kind, name, &params)
		})
	}
	return JSONProblemf(ctx, http.StatusNotFound, "Not found", "object not found: %s", objPath)
}

// configBeingDeleted says the local instance monitor of the object purges or
// deletes it: its config is not served, a peer installing it bringing back
// an instance the orchestration deletes, as v2 refused to fetch a config
// then.
func (a *DaemonAPI) configBeingDeleted(p naming.Path) bool {
	mon := instance.MonitorData.GetByPathAndNode(p, a.localhost)
	if mon == nil {
		return false
	}
	switch mon.GlobalExpect {
	case instance.MonitorGlobalExpectPurged, instance.MonitorGlobalExpectDeleted:
		return true
	default:
		return false
	}
}

// announceConfigServed says the config of the object was served to the
// requester, when it is a peer: it installs it, an instance of it coming.
func (a *DaemonAPI) announceConfigServed(p naming.Path, requester string) {
	if requester == a.localhost || !clusternode.Has(requester) {
		return
	}
	a.Bus.Pub(&msgbus.InstanceConfigServed{Path: p, Node: requester},
		pubsub.Label{"namespace", p.Namespace}, pubsub.Label{"path", p.String()}, a.LabelLocalhost, labelOriginAPI)
}
