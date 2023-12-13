package daemonapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/file"
)

func (a *DaemonApi) GetObjectFile(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
	logName := "GetObjectFile"
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
	resp := api.ObjectFile{
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

func (a DaemonApi) PostObjectFile(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Bad request path", fmt.Sprint(err))
	}
	if p.Exists() {
		return JSONProblemf(ctx, http.StatusConflict, "Conflict", "Use the PUT method instead of POST to update the object config")
	}
	return a.writeObjectFile(ctx, p)
}

func (a DaemonApi) PutObjectFile(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Bad request path", fmt.Sprint(err))
	}
	if !p.Exists() {
		return JSONProblemf(ctx, http.StatusNotFound, "Not found", "Use the POST method instead of PUT to create the object")
	}
	return a.writeObjectFile(ctx, p)
}

func (a DaemonApi) writeObjectFile(ctx echo.Context, p naming.Path) error {
	var body api.PutObjectFileJSONRequestBody
	dec := json.NewDecoder(ctx.Request().Body)
	if err := dec.Decode(&body); err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Bad request body", fmt.Sprint(err))
	}
	o, err := object.New(p, object.WithConfigData(body.Data))
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New object", fmt.Sprint(err))
	}
	configurer := o.(object.Configurer)
	if report, err := configurer.ValidateConfig(ctx.Request().Context()); err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid configuration", fmt.Sprint(report))
	}
	// Use the non-validating commit func as we already validate to emit a explicit error
	if err := configurer.Config().RecommitInvalid(); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Commit", fmt.Sprint(err))
	}
	return ctx.JSON(http.StatusNoContent, nil)
}
