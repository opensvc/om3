package oxcmd

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
	"github.com/opensvc/om3/v3/util/duration"
	"github.com/opensvc/om3/v3/util/hostname"
)

type (
	CmdDaemonSessionList struct {
		OptsGlobal
		NodeSelector    string
		States          []string
		OrchestrationID string
		ExecID          string
		ID              string
	}
)

func (t *CmdDaemonSessionList) Run() error {
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

	items, errs := t.gather(c, nodenames)

	// An id one node no longer holds is not an error when another still
	// does: an action submitted to several nodes is one session per node,
	// and asking for it by id is asking every node that may have run it.
	if t.ID != "" && len(items) == 0 {
		if errs != nil {
			return errs
		}
		return fmt.Errorf("session %s is no longer known on %s: it ended long enough ago to have been dropped, or never ran there",
			t.ID, t.NodeSelector)
	}
	t.render(items)
	return errs
}

// gather asks every node, and returns what they answered together with what
// went wrong asking. A node that has forgotten the id is not one of the
// things that went wrong.
func (t *CmdDaemonSessionList) gather(c *client.T, nodenames []string) ([]api.SessionItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var (
		mu    sync.Mutex
		items []api.SessionItem
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

func (t *CmdDaemonSessionList) one(ctx context.Context, c *client.T, nodename string) ([]api.SessionItem, error) {
	if t.ID != "" {
		resp, err := c.GetDaemonSessionWithResponse(ctx, nodename, t.ID)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", nodename, err)
		}
		switch resp.StatusCode() {
		case http.StatusOK:
			// Several when the command reached several objects of the node.
			return resp.JSON200.Items, nil
		case http.StatusGone:
			// This node has forgotten it, or never ran it. Another may hold
			// it, and saying so here would make asking every node an error
			// wherever one of them answers.
			return nil, nil
		default:
			return nil, fmt.Errorf("%s: %s", nodename, resp.Status())
		}
	}

	params := api.GetDaemonSessionsParams{}
	if len(t.States) > 0 {
		params.States = &t.States
	}
	if t.OrchestrationID != "" {
		params.OrchestrationID = &t.OrchestrationID
	}
	if t.ExecID != "" {
		params.ExecID = &t.ExecID
	}
	resp, err := c.GetDaemonSessionsWithResponse(ctx, nodename, &params)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", nodename, err)
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", nodename, resp.Status())
	}
	return resp.JSON200.Items, nil
}

func (t *CmdDaemonSessionList) render(items []api.SessionItem) {
	output.Renderer{
		DefaultOutput: "tab=NODE:node,STATE:state,ID:id,EXEC_ID:exec_id,PATH:path,ORIGIN:origin,BEGIN_AT:begin_at,DURATION:duration,COMMAND:command",
		Output:        t.Output,
		Color:         t.Color,
		Data:          toSessionViews(items),
		Colorize:      rawconfig.Colorize,
	}.Print()
}

// sessionView is what the table shows: the api reports a duration in
// nanoseconds and an instant to the nanosecond, which a machine wants and a
// reader does not.
type sessionView struct {
	Node            string `json:"node"`
	State           string `json:"state"`
	ID              string `json:"id"`
	OrchestrationID string `json:"orchestration_id,omitempty"`
	Path            string `json:"path,omitempty"`
	Origin          string `json:"origin,omitempty"`
	BeginAt         string `json:"begin_at"`
	Duration        string `json:"duration,omitempty"`
	Command         string `json:"command,omitempty"`
	ExecID          string `json:"exec_id,omitempty"`
	Error           string `json:"error,omitempty"`
}

func toSessionViews(items []api.SessionItem) []sessionView {
	l := make([]sessionView, 0, len(items))
	for _, i := range items {
		v := sessionView{
			Node:    i.Node,
			State:   i.State,
			ID:      i.Id,
			BeginAt: i.BeginAt.Truncate(time.Second).Format(time.RFC3339),
			Command: i.Command,
			Origin:  i.Origin,
			ExecID:  i.ExecId,
		}
		if i.OrchestrationId != nil {
			v.OrchestrationID = *i.OrchestrationId
		}
		if i.Path != nil {
			v.Path = *i.Path
		}
		if i.Error != nil {
			v.Error = *i.Error
		}
		if i.Duration != nil {
			v.Duration = duration.FmtShortDuration(time.Duration(*i.Duration))
		}
		l = append(l, v)
	}
	return l
}
