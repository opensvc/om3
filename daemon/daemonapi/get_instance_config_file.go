package daemonapi

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/file"
)

func (a *DaemonAPI) GetInstanceConfigFile(ctx echo.Context, nodename, namespace string, kind naming.Kind, name string) error {
	objPath, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "invalid path: %s", err)
	}
	redact, ok, err := configReadAccess(ctx, objPath)
	if !ok {
		return err
	}
	nodename = a.parseNodename(nodename)
	if a.localhost == nodename {
		logName := "GetInstanceConfigFile"
		log := LogHandler(ctx, logName)
		log.Tracef("%s: starting", logName)
		log = naming.LogWithPath(log, objPath)

		filename := objPath.ConfigFile()
		mtime := file.ModTime(filename)
		if !mtime.IsZero() {
			ctx.Response().Header().Add(api.HeaderLastModified, mtime.Format(time.RFC3339Nano))
			log.Infof("serve config file %s to %s", objPath, userFromContext(ctx).Username)
			return serveConfigFile(ctx, objPath, filename, redact)
		}
		return JSONProblemf(ctx, http.StatusNotFound, "Not found", "Config file not found for %s@%s", objPath, a.localhost)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.GetInstanceConfigFile(ctx.Request().Context(), nodename, namespace, kind, name)
	})
}
