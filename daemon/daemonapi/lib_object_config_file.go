package daemonapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/api"
)

func (a DaemonApi) writeObjectConfigFile(ctx echo.Context, p naming.Path) error {
	var body api.PutObjectConfigFileJSONRequestBody
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
	return ctx.NoContent(http.StatusNoContent)
}
