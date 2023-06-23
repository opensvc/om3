package daemonapi

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/event"
	"github.com/opensvc/om3/core/event/sseevent"
	"github.com/opensvc/om3/core/slog"
	"github.com/opensvc/om3/daemon/api"
)

// GetNodeLogs feeds publications in rss format.
func (a *DaemonApi) GetNodeLogs(ctx echo.Context, params api.GetNodeLogsParams) error {
	if err := assertRoleRoot(ctx); err != nil {
		return err
	}
	var (
		handlerName = "GetNodeLogs"
	)
	log := LogHandler(ctx, handlerName)
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

	name := fmt.Sprintf("lsnr-handler-log %s from %s %s", handlerName, ctx.Request().RemoteAddr, ctx.Get("uuid"))
	if params.Filter != nil && len(*params.Filter) > 0 {
		name += " filters: [" + strings.Join(*params.Filter, " ") + "]"
	}

	stream, err := slog.GetEventStreamFromNode(filters)
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

// parseLogFilters return filters from b.Filter
func parseLogFilters(l *[]string) (map[string]any, error) {
	filters := make(map[string]any)
	if l == nil {
		return filters, nil
	}
	for _, s := range *l {
		k, v, err := parseLogFilter(s)
		if err != nil {
			return nil, err
		}
		filters[k] = v
	}
	return filters, nil
}

// parseLogFilter return filter from s
//
// filter syntax is: label=value[,label=value]*
func parseLogFilter(s string) (string, string, error) {
	splitted := strings.SplitN(s, "=", 2)
	if len(splitted) == 2 {
		return splitted[0], splitted[1], nil
	} else {
		return "", "", fmt.Errorf("invalid filter expression: %s", s)
	}
}
