package daemonapi

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/file"
)

func (a *DaemonAPI) GetObjectConfigFile(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
	logName := "GetObjectConfigFile"
	log := LogHandler(ctx, logName)
	log.Debugf("%s: starting", logName)

	objPath, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		log.Warnf("%s: %s", logName, err)
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "invalid path: %s", err)
	}

	log = naming.LogWithPath(log, objPath)

	filename := objPath.ConfigFile()

	mtime := file.ModTime(filename)
	if mtime.IsZero() {
		log.Infof("%s: configFile no present(mtime) %s %s", logName, filename, mtime)
		return JSONProblemf(ctx, http.StatusNotFound, "Not found", "configFile no present(mtime) %s %s", filename, mtime)
	}

	ctx.Response().Header().Add(api.HeaderLastModifiedNano, mtime.Format(time.RFC3339Nano))
	log.Infof("serve config file %s to %s", objPath, userFromContext(ctx).GetUserName())
	return ctx.File(filename)
}
