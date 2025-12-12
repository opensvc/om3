package daemonapi

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/core/event"
	"github.com/opensvc/om3/v3/core/event/sseevent"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/streamlog"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/rbac"
)

// GetNodeLogs feeds publications in rss format.
func (a *DaemonAPI) GetNodeLogs(ctx echo.Context, nodename string, params api.GetNodeLogsParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	if nodename == a.localhost || nodename == "localhost" {
		return a.getLocalNodeLogs(ctx, params)
	} else if !clusternode.Has(nodename) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid nodename", "field 'nodename' with value '%s' is not a cluster node", nodename)
	} else {
		return a.getPeerNodeLogs(ctx, nodename, params)
	}
}

func (a *DaemonAPI) getPeerNodeLogs(ctx echo.Context, nodename string, params api.GetNodeLogsParams) error {
	log := LogHandler(ctx, "GetNodeLogs")
	evCtx := ctx.Request().Context()
	request := ctx.Request()

	c, err := a.newProxyClient(ctx, nodename, client.WithTimeout(0))
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
	}

	w := ctx.Response()
	resp, err := c.GetNodeLogs(evCtx, nodename, &params)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
	} else if resp.StatusCode != http.StatusOK {
		if _, err := io.Copy(w, resp.Body); err != nil {
			return err
		}
	}
	if request.Header.Get("accept") == "text/event-stream" {
		setStreamHeaders(w)
	}

	w.WriteHeader(http.StatusOK)

	// don't wait first event to flush response
	w.Flush()

	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Errorf("response from %s body close: %s", nodename, err)
		}
	}()
	var follow bool
	if params.Follow != nil && *params.Follow {
		follow = true
	}
	if !follow {
		if _, err := io.Copy(w, resp.Body); err != nil {
			return err
		}
		w.Flush()
	} else {
		for {
			if _, err := io.Copy(w, resp.Body); err != nil {
				return err
			}
			w.Flush()
		}
	}
	return nil
}

func (a *DaemonAPI) getLocalNodeLogs(ctx echo.Context, params api.GetNodeLogsParams) error {
	if v, err := assertRole(ctx, rbac.RoleRoot); err != nil {
		return err
	} else if !v {
		return nil
	}
	var (
		handlerName = "GetNodeLogs"
	)
	log := LogHandler(ctx, handlerName)
	log.Tracef("starting")
	defer log.Tracef("done")

	matches, err := parseLogFilters(params.Filter)
	if err != nil {
		log.Infof("invalid parameter: field 'filter' with value '%s' validation error: %s", *params.Filter, err)
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "field 'filter' with value '%s' validation error: %s", *params.Filter, err)
	}
	if params.Paths != nil {
		paths, err := naming.ParsePaths(*params.Paths...)
		if err != nil {
			return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "error parsing paths: %s error: %s", *params.Paths, err)
		}
		matches = append(matches, filtersFromPaths(paths)...)
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
			log.Tracef("stream.Stop: %s", err)
		}
	}()
	w.WriteHeader(http.StatusOK)

	// don't wait first event to flush response
	w.Flush()

	sseWriter := sseevent.NewWriter(w)
	for {
		select {
		case <-ctx.Request().Context().Done():
			return nil
		case ev := <-stream.Events():
			if _, err := sseWriter.Write(&event.Event{Kind: "log", Data: ev.B}); err != nil {
				log.Tracef("sseWriter.Write: %s", err)
				break
			}
			w.Flush()
		case err := <-stream.Errors():
			if err == nil {
				return nil
			}
			log.Tracef("stream.Error: %s", err)
		}
	}
}

// parseLogFilters return filters from b.Filter
func parseLogFilters(l *[]string) ([]string, error) {
	filters := make([]string, 0)
	if l == nil {
		return filters, nil
	}
	for _, s := range *l {
		if s == "" {
			continue
		}
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
	split := strings.SplitN(s, "=", 2)
	if len(split) == 2 {
		return split[0], split[1], nil
	} else {
		return "", "", fmt.Errorf("invalid filter expression: %s", s)
	}
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
