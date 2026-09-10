package omcmd

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/opensvc/om3/v3/core/client"
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
	nodename := t.NodeSelector
	if nodename == "" {
		nodename = hostname.Hostname()
	}

	if t.ID != "" {
		return t.one(c, nodename)
	}

	params := api.GetDaemonOrchestrationsParams{}
	if len(t.States) > 0 {
		params.States = &t.States
	}
	resp, err := c.GetDaemonOrchestrationsWithResponse(context.Background(), nodename, &params)
	if err != nil {
		return err
	}
	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("%s: %s", nodename, resp.Status())
	}
	t.render(resp.JSON200.Items)
	return nil
}

// one asks for a single orchestration, where the daemon having forgotten it is an
// answer of its own: it is not the same as never having run it, and a client
// polling for the end of an action must not read it as one.
func (t *CmdDaemonOrchestrationList) one(c *client.T, nodename string) error {
	resp, err := c.GetDaemonOrchestrationWithResponse(context.Background(), nodename, t.ID)
	if err != nil {
		return err
	}
	switch resp.StatusCode() {
	case http.StatusOK:
		t.render([]api.OrchestrationItem{*resp.JSON200})
		return nil
	case http.StatusGone:
		return fmt.Errorf("%s: orchestration %s is no longer known: it ended long enough ago to have been dropped, or never ran there", nodename, t.ID)
	default:
		return fmt.Errorf("%s: %s", nodename, resp.Status())
	}
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
