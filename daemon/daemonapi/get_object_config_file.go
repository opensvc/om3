package daemonapi

import (
	"net/http"
	"os"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/file"
)

func (a *DaemonApi) GetObjectConfigFile(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
	logName := "GetObjectConfigFile"
	log := LogHandler(ctx, logName)
	log.Debugf("%s: starting", logName)

	objPath, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		log.Warnf("%s: %s", logName, err)
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "invalid path: %s", err)
	}

	filename := objPath.ConfigFile()

	mtime := file.ModTime(filename)
	if mtime.IsZero() {
		log.Infof("%s: configFile no present(mtime) %s %s", logName, filename, mtime)
		return JSONProblemf(ctx, http.StatusNotFound, "Not found", "configFile no present(mtime) %s %s", filename, mtime)
	}
	resp := api.ObjectConfigFile{
		Mtime: mtime,
	}
	resp.Data, err = os.ReadFile(filename)

	if err != nil {
		log.Infof("%s: readfile %s %s (may be deleted): %s", logName, objPath, filename, err)
		return JSONProblemf(ctx, http.StatusNotFound, "Not found", "readfile %s %s (may be deleted)", objPath, filename)
	}
	if file.ModTime(filename) != resp.Mtime {
		log.Infof("%s: file has changed %s", logName, filename)
		return JSONProblemf(ctx, http.StatusTooEarly, "Too early", "file has changed %s", filename)
	}

	return ctx.JSON(http.StatusOK, resp)
}
