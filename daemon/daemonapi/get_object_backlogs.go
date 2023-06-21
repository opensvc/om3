package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/core/slog"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/daemonauth"
)

// GetObjectBacklogs feeds publications in rss format.
func (a *DaemonApi) GetObjectBacklogs(ctx echo.Context, params api.GetObjectBacklogsParams) error {
	var (
		handlerName = "GetObjectBacklogs"
	)
	log := LogHandler(ctx, handlerName)
	log.Debug().Msg("starting")
	defer log.Debug().Msg("done")

	user := User(ctx)
	grants := Grants(user)
	if !grants.HasAnyRole(daemonauth.RoleRoot, daemonauth.RoleJoin) {
		log.Info().Msg("not allowed, need at least 'root' or 'join' grant")
		return ctx.NoContent(http.StatusForbidden)
	}

	filters, err := parseLogFilters(params.Filter)
	if err != nil {
		log.Info().Err(err).Msgf("Invalid parameter: field 'filter' with value '%s' validation error", *params.Filter)
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "field 'filter' with value '%s' validation error: %s", *params.Filter, err)
	}

	paths, err := path.ParseList(params.Paths...)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "error parsing paths: %s", params.Paths, err)
	}
	events, err := slog.GetEventsFromObjects(paths, filters)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), "%s", err)
	}
	return ctx.JSON(http.StatusOK, events)
}
