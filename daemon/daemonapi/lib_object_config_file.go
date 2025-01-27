package daemonapi

import (
	"io"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
)

func (a *DaemonAPI) writeObjectConfigFile(ctx echo.Context, p naming.Path) error {
	body, err := io.ReadAll(ctx.Request().Body)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Read body", "%s", err)
	}
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
	// Use the non-validating commit func as we already validate to emit an explicit error
	if err := configurer.Config().RecommitInvalid(); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Commit", "%s", err)
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
