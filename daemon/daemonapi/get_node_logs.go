package daemonapi

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/event"
	"github.com/opensvc/om3/core/event/sseevent"
	"github.com/opensvc/om3/core/streamlog"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/rbac"
)

// GetNodeLogs feeds publications in rss format.
func (a *DaemonApi) GetNodeLogs(ctx echo.Context, params api.GetNodeLogsParams) error {
	if v, err := assertRole(ctx, rbac.RoleRoot); err != nil {
		return err
	} else if !v {
		return nil
	}
	var (
		handlerName = "GetNodeLogs"
	)
	log := LogHandler(ctx, handlerName)
	log.Debugf("starting")
	defer log.Debugf("done")

	matches, err := parseLogFilters(params.Filter)
	if err != nil {
		log.Infof("Invalid parameter: field 'filter' with value '%s' validation error: %s", *params.Filter, err)
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
			log.Debugf("stream.Stop: %s", err)
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
			log.Debugf("stream.Error: %s", err)
		}
	}
	return nil
}

// parseLogFilters return filters from b.Filter
func parseLogFilters(l *[]string) ([]string, error) {
	filters := make([]string, 0)
	if l == nil {
		return filters, nil
	}
	for _, s := range *l {
		_, _, err := parseLogFilter(s)
		if err != nil {
			return nil, err
		}
		filters = append(filters, s)
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
