package daemonapi

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/file"
)

func (a *DaemonAPI) GetClusterConfigFile(ctx echo.Context) error {
	logName := "GetClusterConfigFile"
	log := LogHandler(ctx, logName)
	log.Debugf("%s: starting", logName)

	objPath := naming.Cluster
	log = naming.LogWithPath(log, objPath)

	filename := objPath.ConfigFile()
	mtime := file.ModTime(filename)
	if mtime.IsZero() {
		log.Infof("%s: config file not found: %s", logName, filename)
		return JSONProblemf(ctx, http.StatusNotFound, "Not found", "config file not found: %s", filename)
	}

	ctx.Response().Header().Add(api.HeaderLastModifiedNano, mtime.Format(time.RFC3339Nano))
	log.Infof("serve config file %s to %s", objPath, userFromContext(ctx).GetUserName())
	return ctx.File(filename)
}
