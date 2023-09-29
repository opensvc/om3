package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/event"
	"github.com/opensvc/om3/core/event/sseevent"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/slog"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/rbac"
)

func (a *DaemonApi) GetInstanceLogs(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.GetInstanceLogsParams) error {
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "%s", err)
		return err
	}
	return a.GetInstancesLogs(ctx, api.GetInstancesLogsParams{
		Paths:  naming.Paths{p}.StrSlice(),
		Filter: params.Filter,
	})
}

// GetInstancesLogs feeds publications in rss format.
func (a *DaemonApi) GetInstancesLogs(ctx echo.Context, params api.GetInstancesLogsParams) error {
	if v, err := assertRole(ctx, rbac.RoleRoot); err != nil {
		return err
	} else if !v {
		return nil
	}
	log := LogHandler(ctx, "GetInstancesLogs")
	log.Debug().Msg("starting")
	defer log.Debug().Msg("done")

	filters, err := parseLogFilters(params.Filter)
	if err != nil {
		log.Info().Err(err).Msgf("Invalid parameter: field 'filter' with value '%s' validation error", *params.Filter)
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "field 'filter' with value '%s' validation error: %s", *params.Filter, err)
	}

	r := ctx.Request()
	w := ctx.Response()
	if r.Header.Get("accept") == "text/event-stream" {
		setStreamHeaders(w)
	}

	paths, err := naming.ParseList(params.Paths...)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "error parsing paths: %s error: %s", params.Paths, err)
	}
	stream, err := slog.GetEventStreamFromObjects(paths, filters)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), "%s", err)
	}
	defer func() {
		if err := stream.Stop(); err != nil {
			log.Debug().Err(err).Msgf("stream.Stop")
		}
	}()
	w.WriteHeader(http.StatusOK)

	// don't wait first event to flush response
	w.Flush()

	eventC := stream.Events()
	sseWriter := sseevent.NewWriter(w)
	for ev := range eventC {
		if _, err := sseWriter.Write(&event.Event{Kind: "log", Data: ev.B}); err != nil {
			break
		}
		w.Flush()
	}
	return nil
}
