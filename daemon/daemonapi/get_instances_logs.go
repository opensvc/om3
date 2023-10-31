package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/event"
	"github.com/opensvc/om3/core/event/sseevent"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/streamlog"
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
		Follow: params.Follow,
		Lines:  params.Lines,
	})
}

func filtersFromPaths(paths naming.Paths) (filters []string) {
	last := len(paths) - 1
	for i, path := range paths {
		filters = append(filters, "OBJ_PATH="+path.String())
		if i > 0 && i < last {
			filters = append(filters, "+")
		}
	}
	return
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

	matches, err := parseLogFilters(params.Filter)
	if err != nil {
		log.Info().Err(err).Msgf("Invalid parameter: field 'filter' with value '%s' validation error", *params.Filter)
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "field 'filter' with value '%s' validation error: %s", *params.Filter, err)
	}

	r := ctx.Request()
	w := ctx.Response()
	if r.Header.Get("accept") == "text/event-stream" {
		setStreamHeaders(w)
	}

	paths, err := naming.ParsePaths(params.Paths...)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "error parsing paths: %s error: %s", params.Paths, err)
	}

	matches = append(matches, filtersFromPaths(paths)...)
	stream := streamlog.NewStream()
	var follow bool
	if params.Follow != nil {
		follow = *params.Follow
	}
	lines := 50
	if params.Lines != nil {
		lines = *params.Lines
	}
	streamConfig := streamlog.StreamConfig{
		Follow:  follow,
		Lines:   lines,
		Matches: matches,
	}
	if err := stream.Start(streamConfig); err != nil {
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

	sseWriter := sseevent.NewWriter(w)
	for {
		select {
		case ev := <-stream.Events():
			if _, err := sseWriter.Write(&event.Event{Kind: "log", Data: ev.B}); err != nil {
				break
			}
			w.Flush()
		case err := <-stream.Errors():
			if err == nil {
				return nil
			}
			log.Debug().Err(err).Msgf("stream.Error")
		}
	}
	return nil
}
