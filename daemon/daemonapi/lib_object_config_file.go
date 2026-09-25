package daemonapi

import (
	"io"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/file"
)

func (a *DaemonAPI) writeObjectConfigFile(ctx echo.Context, p naming.Path, body []byte) error {
	o, err := object.New(p, object.WithConfigData(body))
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New object", "%s", err)
	}
	configurer := o.(object.Configurer)
	alerts, err := configurer.ValidateConfig(ctx.Request().Context())
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Validate config", "%s", err)
	}
	if alerts.HasError() {
		return JSONProblemf(ctx, http.StatusBadRequest, "Validate config", "%s", err)
	}
	if err := refuseClaimOverrun(ctx.Request().Context(), p, o, configurer.Config()); err != nil {
		return JSONProblemf(ctx, http.StatusForbidden, "Forbidden", "%s", err)
	}
	// Use the non-validating commit func as we already validate to emit an explicit error
	if err := configurer.Config().RecommitInvalid(); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Commit", "%s", err)
	}
	a.announceConfigFileWritten(p)
	warnSharedRootlessAccounts(ctx, p)
	// Answer with the timestamp the configuration now carries, so the caller
	// can require it of the actions it goes on to ask of the instances. This
	// write lands on the peer nodes a moment after it is acknowledged here,
	// and without the timestamp a caller has nothing to name the configuration
	// it just wrote.
	if mtime := file.ModTime(p.ConfigFile()); !mtime.IsZero() {
		ctx.Response().Header().Add(api.HeaderLastModified, mtime.Format(time.RFC3339Nano))
	}
	return ctx.NoContent(http.StatusNoContent)
}

func (a *DaemonAPI) writeNodeConfigFile(ctx echo.Context, nodename string) error {
	body, err := io.ReadAll(ctx.Request().Body)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Read body", "%s", err)
	}
	o, err := object.NewNode(object.WithConfigData(body))
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New object", "%s", err)
	}
	alerts, err := o.ValidateConfig()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Validate config", "%s", err)
	}
	if alerts.HasError() {
		return JSONProblemf(ctx, http.StatusBadRequest, "Validate config", "%s", err)
	}
	// Use the non-validating commit func as we already validate to emit an explicit error
	if err := o.Config().RecommitInvalid(); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Commit", "%s", err)
	}
	return ctx.NoContent(http.StatusNoContent)
}
