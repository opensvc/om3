package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/session"
)

func (a *DaemonAPI) GetDaemonOrchestrations(ctx echo.Context, nodename string, params api.GetDaemonOrchestrationsParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if a.localhost != nodename {
		return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.GetDaemonOrchestrations(ctx.Request().Context(), nodename, &params)
		})
	}

	filter := session.Filter{}
	if params.States != nil {
		for _, s := range *params.States {
			filter.States = append(filter.States, session.State(s))
		}
	}
	if params.Selector != nil {
		filter.Path = *params.Selector
	}

	items := make([]api.OrchestrationItem, 0)
	for _, o := range session.ListOrchestrations(filter) {
		items = append(items, orchestrationItem(o))
	}
	return ctx.JSON(http.StatusOK, api.OrchestrationList{Kind: api.OrchestrationListKindOrchestrationList, Items: items})
}

func (a *DaemonAPI) GetDaemonOrchestration(ctx echo.Context, nodename string, id string) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if a.localhost != nodename {
		return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.GetDaemonOrchestration(ctx.Request().Context(), nodename, id)
		})
	}
	o, ok := session.GetOrchestration(id)
	if !ok {
		// Gone and not NotFound, for the reason GetSession is: the sessions
		// of an orchestration are asked for by filtering on its id, and an
		// empty list means nothing until this has said whether the
		// orchestration itself is still known.
		return JSONProblemf(ctx, http.StatusGone, "Orchestration no longer known",
			"orchestration %s has been dropped, or never ran on this node", id)
	}
	return ctx.JSON(http.StatusOK, orchestrationItem(o))
}

func orchestrationItem(o session.Orchestration) api.OrchestrationItem {
	item := api.OrchestrationItem{
		Id:      o.ID,
		Node:    o.Node,
		State:   string(o.State),
		BeginAt: o.BeginAt,
	}
	if o.Path != "" {
		item.Path = &o.Path
	}
	if o.GlobalExpect != "" {
		item.GlobalExpect = &o.GlobalExpect
	}
	if o.Error != "" {
		item.Error = &o.Error
	}
	if o.EndAt != nil {
		item.EndAt = o.EndAt
	}
	return item
}
