package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/slog"
	"github.com/opensvc/om3/daemon/api"
)

// GetNodeBacklogs feeds publications in rss format.
func (a *DaemonApi) GetNodeBacklogs(ctx echo.Context, params api.GetNodeBacklogsParams) error {
	if err := assertRoleRoot(ctx); err != nil {
		return err
	}
	log := LogHandler(ctx, "GetNodeBacklogs")
	log.Debug().Msg("starting")
	defer log.Debug().Msg("done")

	filters, err := parseLogFilters(params.Filter)
	if err != nil {
		log.Info().Err(err).Msgf("Invalid parameter: field 'filter' with value '%s' validation error", *params.Filter)
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "field 'filter' with value '%s' validation error: %s", *params.Filter, err)
	}
	events, err := slog.GetEventsFromNode(filters)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), "%s", err)
	}
	return ctx.JSON(http.StatusOK, events)
}
