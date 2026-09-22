package daemonapi

import (
	"net/http"
	"os"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/file"
)

func (a *DaemonAPI) GetClusterConfigFile(ctx echo.Context, params api.GetClusterConfigFileParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	logName := "GetClusterConfigFile"
	log := LogHandler(ctx, logName)
	log.Tracef("%s: starting", logName)

	objPath := naming.Cluster
	log = naming.LogWithPath(log, objPath)

	filename := objPath.ConfigFile()
	mtime := file.ModTime(filename)
	if mtime.IsZero() {
		log.Infof("%s: config file not found: %s", logName, filename)
		return JSONProblemf(ctx, http.StatusNotFound, "Not found", "config file not found: %s", filename)
	}

	ctx.Response().Header().Add(api.HeaderLastModified, mtime.Format(time.RFC3339Nano))
	log.Infof("serve config file %s to %s", objPath, userFromContext(ctx).Username)
	if params.RedactSecrets == nil || !*params.RedactSecrets {
		return ctx.File(filename)
	}

	content, err := os.ReadFile(filename)
	if err != nil {
		log.Warnf("failed to read cluster config file: %s", err)
		return JSONProblemf(ctx, http.StatusInternalServerError, "Internal server error", "Failed to read cluster config file")
	}
	b, err := object.RedactSecrets(content, "")
	if err != nil {
		log.Warnf("redact cluster config file: %s", err)
		return JSONProblemf(ctx, http.StatusInternalServerError, "Internal server error", "Failed to redact cluster config file")
	}
	return ctx.Blob(http.StatusOK, "application/octet-stream", b)
}
