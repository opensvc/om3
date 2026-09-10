package omcmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/nodeselector"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/hostname"
)

type (
	CmdDaemonOrchestrationList struct {
		OptsGlobal
		NodeSelector    string
		States          []string
		OrchestrationID string
		ID              string
	}
)

func (t *CmdDaemonOrchestrationList) Run() error {
	c, err := client.New()
	if err != nil {
		return err
	}
	if t.NodeSelector == "" {
		t.NodeSelector = hostname.Hostname()
	}
	nodenames, err := nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand()
	if err != nil {
		return err
	}
	if len(nodenames) == 0 {
		return fmt.Errorf("no node matching %s", t.NodeSelector)
	}

	items, errs := t.merge(t.gather(c, nodenames))

	// Every node of the object answers for an orchestration, so one that
	// has forgotten it is only an answer when they all have.
	if t.ID != "" && len(items) == 0 {
		if errs != nil {
			return errs
		}
		return fmt.Errorf("orchestration %s is no longer known on %s: it ended long enough ago to have been dropped, or never ran there",
			t.ID, t.NodeSelector)
	}
	t.render(items)
	return errs
}

// merge folds the answers of the nodes into one entry per orchestration.
//
// Every node of an object answers for its orchestrations, so asking several
// returns the same orchestration several times. The answer of the node that
// accepted it is the one kept, being the only one that knows which node was
// asked for it.
func (t *CmdDaemonOrchestrationList) merge(items []api.OrchestrationItem, errs error) ([]api.OrchestrationItem, error) {
	byID := make(map[string]api.OrchestrationItem, len(items))
	order := make([]string, 0, len(items))
	for _, i := range items {
		kept, ok := byID[i.Id]
		if !ok {
			byID[i.Id] = i
			order = append(order, i.Id)
			continue
		}
		if kept.Node == "" && i.Node != "" {
			byID[i.Id] = i
		}
	}
	l := make([]api.OrchestrationItem, 0, len(order))
	for _, id := range order {
		l = append(l, byID[id])
	}
	return l, errs
}

// gather asks every node, and returns what they answered together with what
// went wrong asking. A node that has forgotten the id is not one of the
// things that went wrong.
func (t *CmdDaemonOrchestrationList) gather(c *client.T, nodenames []string) ([]api.OrchestrationItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var (
		mu    sync.Mutex
		items []api.OrchestrationItem
		errs  error
		wg    sync.WaitGroup
	)
	for _, nodename := range nodenames {
		wg.Add(1)
		go func(nodename string) {
			defer wg.Done()
			l, err := t.one(ctx, c, nodename)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = errors.Join(errs, err)
				return
			}
			items = append(items, l...)
		}(nodename)
	}
	wg.Wait()
	return items, errs
}

func (t *CmdDaemonOrchestrationList) one(ctx context.Context, c *client.T, nodename string) ([]api.OrchestrationItem, error) {
	if t.ID != "" {
		resp, err := c.GetDaemonOrchestrationWithResponse(ctx, nodename, t.ID)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", nodename, err)
		}
		switch resp.StatusCode() {
		case http.StatusOK:
			return []api.OrchestrationItem{*resp.JSON200}, nil
		case http.StatusGone:
			// This node has forgotten it, or never ran it. Another may hold
			// it, and saying so here would make asking every node an error
			// wherever one of them answers.
			return nil, nil
		default:
			return nil, fmt.Errorf("%s: %s", nodename, resp.Status())
		}
	}

	params := api.GetDaemonOrchestrationsParams{}
	if len(t.States) > 0 {
		params.States = &t.States
	}
	resp, err := c.GetDaemonOrchestrationsWithResponse(ctx, nodename, &params)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", nodename, err)
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", nodename, resp.Status())
	}
	return resp.JSON200.Items, nil
}

func (t *CmdDaemonOrchestrationList) render(items []api.OrchestrationItem) {
	output.Renderer{
		DefaultOutput: "tab=STATE:state,ID:id,PATH:path,GLOBAL_EXPECT:global_expect,ACCEPTED_BY:node,BEGIN_AT:begin_at,END_AT:end_at",
		Output:        t.Output,
		Color:         t.Color,
		Data:          toOrchestrationViews(items),
		Colorize:      rawconfig.Colorize,
	}.Print()
}

// orchestrationView is what the table shows, for the reason sessionView is.
type orchestrationView struct {
	State        string `json:"state"`
	ID           string `json:"id"`
	Path         string `json:"path,omitempty"`
	GlobalExpect string `json:"global_expect,omitempty"`
	AcceptedBy   string `json:"node,omitempty"`
	BeginAt      string `json:"begin_at"`
	EndAt        string `json:"end_at,omitempty"`
	Error        string `json:"error,omitempty"`
}

func toOrchestrationViews(items []api.OrchestrationItem) []orchestrationView {
	l := make([]orchestrationView, 0, len(items))
	for _, i := range items {
		v := orchestrationView{
			State:      i.State,
			ID:         i.Id,
			AcceptedBy: i.Node,
			BeginAt:    i.BeginAt.Truncate(time.Second).Format(time.RFC3339),
		}
		if i.Path != nil {
			v.Path = *i.Path
		}
		if i.GlobalExpect != nil {
			v.GlobalExpect = *i.GlobalExpect
		}
		if i.Error != nil {
			v.Error = *i.Error
		}
		if i.EndAt != nil {
			v.EndAt = i.EndAt.Truncate(time.Second).Format(time.RFC3339)
		}
		l = append(l, v)
	}
	return l
}
