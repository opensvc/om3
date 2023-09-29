package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/slog"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/rbac"
)

func (a *DaemonApi) GetInstanceBacklogs(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.GetInstanceBacklogsParams) error {
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "%s", err)
		return err
	}
	return a.GetInstancesBacklogs(ctx, api.GetInstancesBacklogsParams{
		Paths:  naming.Paths{p}.StrSlice(),
		Filter: params.Filter,
	})
}

// GetInstancesBacklogs feeds publications in rss format.
func (a *DaemonApi) GetInstancesBacklogs(ctx echo.Context, params api.GetInstancesBacklogsParams) error {
	if v, err := assertRole(ctx, rbac.RoleRoot); err != nil {
		return err
	} else if !v {
		return nil
	}
	log := LogHandler(ctx, "GetInstancesBacklogs")
	log.Debug().Msg("starting")
	defer log.Debug().Msg("done")

	filters, err := parseLogFilters(params.Filter)
	if err != nil {
		log.Info().Err(err).Msgf("Invalid parameter: field 'filter' with value '%s' validation error", *params.Filter)
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "field 'filter' with value '%s' validation error: %s", *params.Filter, err)
	}

	paths, err := naming.ParseList(params.Paths...)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "error parsing paths: %s error: %s", params.Paths, err)
	}
	events, err := slog.GetEventsFromObjects(paths, filters)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), "%s", err)
	}
	return ctx.JSON(http.StatusOK, events)
}
