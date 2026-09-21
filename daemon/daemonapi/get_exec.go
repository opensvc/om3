package daemonapi

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/proc"
	"github.com/opensvc/om3/v3/daemon/session"
)

func (a *DaemonAPI) GetDaemonExecs(ctx echo.Context, nodename string, params api.GetDaemonExecsParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if a.localhost != nodename {
		return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.GetDaemonExecs(ctx.Request().Context(), nodename, &params)
		})
	}

	filter := session.Filter{}
	if params.States != nil {
		for _, s := range *params.States {
			filter.States = append(filter.States, session.State(s))
		}
	}
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

	waitCtx, cancel, waiting, err := waitContext(ctx, params.Wait)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	defer cancel()

	var (
		l       []session.Exec
		running bool
	)
	if waiting {
		l, running = session.WaitExecs(waitCtx, filter)
	} else {
		l = session.ListExecs(filter)
	}
	pids := proc.PidByExecID()
	items := make([]api.ExecItem, 0, len(l))
	for _, e := range l {
		items = append(items, execItem(e, pids))
	}
	if running {
		// The wait expired on execs that are still running. They are answered
		// all the same, so the caller sees what it is waiting for, and the
		// status says the wait is what ended, not them.
		return ctx.JSON(http.StatusRequestTimeout, api.ExecList{Kind: api.ExecListKindExecList, Items: items})
	}
	return ctx.JSON(http.StatusOK, api.ExecList{Kind: api.ExecListKindExecList, Items: items})
}

func (a *DaemonAPI) GetDaemonExec(ctx echo.Context, nodename string, execID uuid.UUID, params api.GetDaemonExecParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if a.localhost != nodename {
		return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.GetDaemonExec(ctx.Request().Context(), nodename, execID, &params)
		})
	}
	waitCtx, cancel, waiting, err := waitContext(ctx, params.Wait)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	defer cancel()

	var (
		e  session.Exec
		ok bool
	)
	if waiting {
		e, ok = session.WaitExec(waitCtx, execID.String())
	} else {
		e, ok = session.GetExec(execID.String())
	}
	if waiting && ok && e.EndedAt == nil {
		return JSONProblemf(ctx, http.StatusRequestTimeout, "Exec is still running",
			"exec %s has not ended before the wait expired", execID)
	}
	if !ok {
		// Gone and not NotFound: the daemon may well have run this exec and
		// dropped it since, and a client polling for the end of what it
		// submitted must not read the answer as "never happened".
		return JSONProblemf(ctx, http.StatusGone, "Exec no longer known",
			"exec %s has been dropped, or never ran on this node", execID)
	}
	return ctx.JSON(http.StatusOK, execItem(e, proc.PidByExecID()))
}

// execItem renders one exec, with the pid of the process running it when one
// still is. The exec store remembers what the exec is; the process table
// knows only whether it is still running and under which pid, so the two are
// joined on the exec id rather than kept as two shapes of the same thing.
func execItem(e session.Exec, pids map[string]int) api.ExecItem {
	item := api.ExecItem{
		SessionID: e.SessionID,
		ExecID:    e.ExecID,
		Node:      e.Node,
		Origin:    e.Origin,
		Command:   e.Command,
		State:     string(e.State),
		StartedAt: e.StartedAt,
		ExitCode:  e.ExitCode,
	}
	if e.OrchestrationID != "" {
		item.OrchestrationID = &e.OrchestrationID
	}
	if e.Path != "" {
		item.Path = &e.Path
	}
	if e.RID != "" {
		item.RID = &e.RID
	}
	if e.Title != "" {
		item.Title = &e.Title
	}
	if e.Error != "" {
		item.Error = &e.Error
	}
	if e.EndedAt != nil {
		item.EndedAt = e.EndedAt
	}
	if pid, ok := pids[e.ExecID]; ok {
		item.Pid = &pid
	}
	return item
}
