package daemonapi

import (
	"net/http"
	"os"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/opensvc/om3/v3/core/object"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/file"
)

func (a *DaemonAPI) GetObjectConfigFile(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.GetObjectConfigFileParams) error {
	if v, err := assertGuest(ctx, namespace); !v {
		return err
	}

	logName := "GetObjectConfigFile"
	log := LogHandler(ctx, logName)
	log.Tracef("%s: starting", logName)

	objPath, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		log.Warnf("%s: %s", logName, err)
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "invalid path: %s", err)
	}
	log = naming.LogWithPath(log, objPath)

	if instMon := instance.ConfigData.GetByPathAndNode(objPath, a.localhost); instMon != nil {
		filename := objPath.ConfigFile()
		mtime := file.ModTime(filename)
		if mtime.IsZero() {
			log.Infof("%s: config file not found: %s", logName, filename)
			return JSONProblemf(ctx, http.StatusNotFound, "Not found", "config file not found: %s", filename)
		}

		ctx.Response().Header().Add(api.HeaderLastModified, mtime.Format(time.RFC3339Nano))
		log.Infof("serve config file %s to %s", objPath, userFromContext(ctx).GetUserName())

		content, err := os.ReadFile(filename)
		if err != nil {
			log.Warnf("%s: failed to read config file %s: %s", logName, filename, err)
			return JSONProblemf(ctx, http.StatusInternalServerError, "Internal server error", "Failed to read config file: %s", filename)
		}

		b := object.RedactSecrets(content, kind.String())
		return ctx.Blob(http.StatusOK, "application/octet-stream", b)
	}
	for nodename := range instance.ConfigData.GetByPath(objPath) {
		return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.GetObjectConfigFile(ctx.Request().Context(), namespace, kind, name, &params)
		})
	}
	return JSONProblemf(ctx, http.StatusNotFound, "Not found", "object not found: %s", objPath)
}
