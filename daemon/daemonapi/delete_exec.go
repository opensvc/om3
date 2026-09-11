package daemonapi

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"syscall"

	"github.com/labstack/echo/v4"
	"golang.org/x/sys/unix"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/proc"
	"github.com/opensvc/om3/v3/daemon/session"
)

func (a *DaemonAPI) DeleteDaemonExecs(ctx echo.Context, nodename string, params api.DeleteDaemonExecsParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if a.localhost != nodename {
		return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.DeleteDaemonExecs(ctx.Request().Context(), nodename, &params)
		})
	}
	return a.deleteLocalDaemonExecs(ctx, params)
}

func (a *DaemonAPI) deleteLocalDaemonExecs(ctx echo.Context, params api.DeleteDaemonExecsParams) error {
	// Signaling every exec of a node is not something a caller does by
	// omission. The state is not a narrowing for this purpose: only a running
	// exec has a process, so asking for the running ones is asking for all of
	// them.
	narrowed := params.ExecID != nil ||
		params.SessionID != nil ||
		params.OrchestrationID != nil ||
		(params.Origins != nil && len(*params.Origins) > 0) ||
		(params.RID != nil && *params.RID != "") ||
		(params.Selector != nil && *params.Selector != "")
	if !narrowed {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters",
			"name what to signal: one of exec_id, session_id, orchestration_id, rid, origin or selector")
	}

	sig := syscall.SIGKILL
	if params.Signal != nil && *params.Signal != "" {
		var err error
		if sig, err = parseSignal(*params.Signal); err != nil {
			return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "%s", err)
		}
	}

	// Only a running exec has a process to signal.
	filter := session.Filter{States: []session.State{session.StateRunning}}
	if params.Origins != nil {
		filter.Origins = *params.Origins
	}
	if params.SessionID != nil {
		filter.SessionID = params.SessionID.String()
	}
	if params.OrchestrationID != nil {
		filter.OrchestrationID = params.OrchestrationID.String()
	}
	if params.ExecID != nil {
		filter.ExecID = params.ExecID.String()
	}
	if params.RID != nil {
		filter.RID = *params.RID
	}
	if params.Selector != nil {
		filter.Path = *params.Selector
	}

	// The exec is resolved to its pid here, under the daemon's own lock, and
	// never the other way round. A pid a client read from an earlier listing
	// may have exited and been recycled since, and the exec it now names is
	// not the one the client meant.
	byExecID := proc.PidByExecID()
	selected := make([]session.Exec, 0)
	for _, e := range session.ListExecs(filter) {
		if _, ok := byExecID[e.ExecID]; !ok {
			// Running as far as the store knows, but the daemon has no
			// process for it. Nothing to signal.
			continue
		}
		selected = append(selected, e)
	}

	dryRun := params.DryRun != nil && *params.DryRun
	items := make([]api.ExecItem, 0, len(selected))
	for _, e := range selected {
		if !dryRun {
			pid := byExecID[e.ExecID]
			if err := syscall.Kill(pid, sig); err != nil && !errors.Is(err, syscall.ESRCH) {
				// ESRCH is the exec ending between the selection and the
				// signal, which is the outcome asked for.
				return JSONProblemf(ctx, http.StatusInternalServerError, "Signal exec",
					"exec %s, pid %d: %s", e.ExecID, pid, err)
			}
		}
		items = append(items, execItem(e, byExecID))
	}
	return ctx.JSON(http.StatusOK, api.ExecList{Kind: api.ExecListKindExecList, Items: items})
}

// parseSignal returns the signal designated by s, given as a number (9), a
// name (KILL) or a prefixed name (SIGKILL), case insensitive.
func parseSignal(s string) (syscall.Signal, error) {
	s = strings.TrimSpace(s)
	if num, err := strconv.Atoi(s); err == nil {
		sig := syscall.Signal(num)
		if unix.SignalName(sig) == "" {
			return 0, fmt.Errorf("unknown signal %s", s)
		}
		return sig, nil
	}
	name := strings.ToUpper(s)
	if !strings.HasPrefix(name, "SIG") {
		name = "SIG" + name
	}
	sig := unix.SignalNum(name)
	if sig == 0 {
		return 0, fmt.Errorf("unknown signal %s", s)
	}
	return sig, nil
}
