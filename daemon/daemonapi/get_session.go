package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/session"
)

func (a *DaemonAPI) GetDaemonSessions(ctx echo.Context, nodename string, params api.GetDaemonSessionsParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if a.localhost != nodename {
		return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.GetDaemonSessions(ctx.Request().Context(), nodename, &params)
		})
	}

	filter := session.Filter{}
	if params.States != nil {
		for _, s := range *params.States {
			filter.States = append(filter.States, session.State(s))
		}
	}
	if params.OrchestrationID != nil {
		filter.OrchestrationID = *params.OrchestrationID
	}
	if params.Selector != nil {
		filter.Path = *params.Selector
	}

	items := make([]api.SessionItem, 0)
	for _, s := range session.ListSessions(filter) {
		items = append(items, sessionItem(s))
	}
	return ctx.JSON(http.StatusOK, api.SessionList{Kind: api.SessionListKindSessionList, Items: items})
}

func (a *DaemonAPI) GetDaemonSession(ctx echo.Context, nodename string, id string) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if a.localhost != nodename {
		return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.GetDaemonSession(ctx.Request().Context(), nodename, id)
		})
	}
	l := session.GetSessions(id)
	if len(l) == 0 {
		// Gone and not NotFound: the daemon may well have run this session
		// and dropped it since, and a client polling for the end of what it
		// submitted must not read the answer as "never happened".
		return JSONProblemf(ctx, http.StatusGone, "Session no longer known",
			"session %s has been dropped, or never ran on this node", id)
	}
	// Several when the command reached several objects of this node, each of
	// them an exec of its own under the one session id.
	items := make([]api.SessionItem, 0, len(l))
	for _, s := range l {
		items = append(items, sessionItem(s))
	}
	return ctx.JSON(http.StatusOK, api.SessionList{Kind: api.SessionListKindSessionList, Items: items})
}

func sessionItem(s session.Session) api.SessionItem {
	item := api.SessionItem{
		Id:      s.ID,
		ExecId:  s.ExecID,
		Node:    s.Node,
		Origin:  s.Origin,
		Command: s.Command,
		State:   string(s.State),
		BeginAt: s.BeginAt,
	}
	if s.OrchestrationID != "" {
		item.OrchestrationId = &s.OrchestrationID
	}
	if s.Path != "" {
		item.Path = &s.Path
	}
	if s.Title != "" {
		item.Title = &s.Title
	}
	if s.Error != "" {
		item.Error = &s.Error
	}
	if s.EndAt != nil {
		item.EndAt = s.EndAt
		duration := int64(s.Duration)
		item.Duration = &duration
	}
	return item
}
