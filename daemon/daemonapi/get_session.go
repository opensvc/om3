package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/session"
)

func (a *DaemonAPI) GetSessions(ctx echo.Context, nodename string, params api.GetSessionsParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if a.localhost != nodename {
		return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.GetSessions(ctx.Request().Context(), nodename, &params)
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

func (a *DaemonAPI) GetSession(ctx echo.Context, nodename string, id string) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if a.localhost != nodename {
		return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.GetSession(ctx.Request().Context(), nodename, id)
		})
	}
	s, ok := session.GetSession(id)
	if !ok {
		// Gone and not NotFound: the daemon may well have run this session
		// and dropped it since, and a client polling for the end of what it
		// submitted must not read the answer as "never happened".
		return JSONProblemf(ctx, http.StatusGone, "Session no longer known",
			"session %s has been dropped, or never ran on this node", id)
	}
	return ctx.JSON(http.StatusOK, sessionItem(s))
}

func sessionItem(s session.Session) api.SessionItem {
	item := api.SessionItem{
		Id:      s.ID,
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
