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

	l := session.ListExecs(filter)
	pids := proc.PidByExecID()
	items := make([]api.ExecItem, 0, len(l))
	for _, e := range l {
		items = append(items, execItem(e, pids))
	}
	return ctx.JSON(http.StatusOK, api.ExecList{Kind: api.ExecListKindExecList, Items: items})
}

func (a *DaemonAPI) GetDaemonExec(ctx echo.Context, nodename string, execID uuid.UUID) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if a.localhost != nodename {
		return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.GetDaemonExec(ctx.Request().Context(), nodename, execID)
		})
	}
	e, ok := session.GetExec(execID.String())
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
	if e.Duration > 0 {
		d := int64(e.Duration)
		item.Duration = &d
	}
	if pid, ok := pids[e.ExecID]; ok {
		item.Pid = &pid
	}
	return item
}
