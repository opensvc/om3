package daemonapi

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/event"
	"github.com/opensvc/om3/core/event/sseevent"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/core/slog"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/daemonauth"
)

// GetObjectLogs feeds publications in rss format.
func (a *DaemonApi) GetObjectLogs(ctx echo.Context, params api.GetObjectLogsParams) error {
	var (
		handlerName = "GetObjectLogs"
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

	r := ctx.Request()
	w := ctx.Response()
	if r.Header.Get("accept") == "text/event-stream" {
		setStreamHeaders(w)
	}

	name := fmt.Sprintf("lsnr-handler-log %s from %s %s", handlerName, ctx.Request().RemoteAddr, ctx.Get("uuid"))
	if params.Filter != nil && len(*params.Filter) > 0 {
		name += " filters: [" + strings.Join(*params.Filter, " ") + "]"
	}

	paths, err := path.ParseList(params.Paths...)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "error parsing paths: %s", params.Paths, err)
	}
	stream, err := slog.GetEventStreamFromObjects(paths, filters)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), "%s", err)
	}
	defer stream.Stop()
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
