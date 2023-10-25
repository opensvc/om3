package daemonapi

import (
	"net/http"
	"os"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/file"
)

func (a *DaemonApi) GetObjectFile(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
	logName := "GetObjectFile"
	log := LogHandler(ctx, logName)
	log.Debug().Msgf("daemon: api: %s: starting", logName)

	objPath, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		log.Warn().Err(err).Msgf("daemon: api: %s: %s", logName, err)
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "invalid path: %s", err)
	}

	filename := objPath.ConfigFile()

	mtime := file.ModTime(filename)
	if mtime.IsZero() {
		log.Info().Msgf("daemon: api: %s: configFile no present(mtime) %s %s", logName, filename, mtime)
		return JSONProblemf(ctx, http.StatusNotFound, "Not found", "configFile no present(mtime) %s %s", filename, mtime)
	}
	resp := api.ObjectFile{
		Mtime: mtime,
	}
	resp.Data, err = os.ReadFile(filename)

	if err != nil {
		log.Info().Err(err).Msgf("daemon: api: %s: readfile %s %s (may be deleted): %s", logName, objPath, filename, err)
		return JSONProblemf(ctx, http.StatusNotFound, "Not found", "readfile %s %s (may be deleted)", objPath, filename)
	}
	if file.ModTime(filename) != resp.Mtime {
		log.Info().Msgf("daemon: api: %s: file has changed %s", logName, filename)
		return JSONProblemf(ctx, http.StatusTooEarly, "Too early", "file has changed %s", filename)
	}

	return ctx.JSON(http.StatusOK, resp)
}
