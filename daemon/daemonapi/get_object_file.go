package daemonapi

import (
	"net/http"
	"os"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/file"
)

func (a *DaemonApi) GetObjectFile(ctx echo.Context, namespace, kind, name string) error {
	log := LogHandler(ctx, "objecthandler.GetObjectFile")
	log.Debug().Msg("starting")

	objPath, err := path.New(name, namespace, kind)
	if err != nil {
		log.Warn().Err(err).Send()
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "invalid path: %s", err)
	}

	filename := objPath.ConfigFile()

	mtime := file.ModTime(filename)
	if mtime.IsZero() {
		log.Info().Msgf("configFile no present(mtime) %s %s", filename, mtime)
		return JSONProblemf(ctx, http.StatusNotFound, "Not found", "configFile no present(mtime) %s %s", filename, mtime)
	}
	resp := api.ObjectFile{
		Mtime: mtime,
	}
	resp.Data, err = os.ReadFile(filename)

	if err != nil {
		log.Info().Err(err).Msgf("readfile %s %s (may be deleted)", objPath, filename)
		return JSONProblemf(ctx, http.StatusNotFound, "Not found", "readfile %s %s (may be deleted)", objPath, filename)
	}
	if file.ModTime(filename) != resp.Mtime {
		log.Info().Msgf("file has changed %s", filename)
		return JSONProblemf(ctx, http.StatusTooEarly, "Too early", "file has changed %s", filename)
	}

	return ctx.JSON(http.StatusOK, resp)
}
