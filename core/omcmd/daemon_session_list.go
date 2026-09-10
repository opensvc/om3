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
	"github.com/opensvc/om3/v3/util/duration"
	"github.com/opensvc/om3/v3/util/hostname"
)

type (
	CmdDaemonSessionList struct {
		OptsGlobal
		NodeSelector    string
		States          []string
		OrchestrationID string
		ID              string
	}
)

func (t *CmdDaemonSessionList) Run() error {
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

	params := api.GetDaemonSessionsParams{}
	if len(t.States) > 0 {
		params.States = &t.States
	}
	if t.OrchestrationID != "" {
		params.OrchestrationID = &t.OrchestrationID
	}
	resp, err := c.GetDaemonSessionsWithResponse(context.Background(), nodename, &params)
	if err != nil {
		return err
	}
	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("%s: %s", nodename, resp.Status())
	}
	t.render(resp.JSON200.Items)
	return nil
}

// one asks for a single session, where the daemon having forgotten it is an
// answer of its own: it is not the same as never having run it, and a client
// polling for the end of an action must not read it as one.
func (t *CmdDaemonSessionList) one(c *client.T, nodename string) error {
	resp, err := c.GetDaemonSessionWithResponse(context.Background(), nodename, t.ID)
	if err != nil {
		return err
	}
	switch resp.StatusCode() {
	case http.StatusOK:
		t.render([]api.SessionItem{*resp.JSON200})
		return nil
	case http.StatusGone:
		return fmt.Errorf("%s: session %s is no longer known: it ended long enough ago to have been dropped, or never ran there", nodename, t.ID)
	default:
		return fmt.Errorf("%s: %s", nodename, resp.Status())
	}
}

func (t *CmdDaemonSessionList) render(items []api.SessionItem) {
	output.Renderer{
		DefaultOutput: "tab=NODE:node,STATE:state,ID:id,PATH:path,ORIGIN:origin,BEGIN_AT:begin_at,DURATION:duration,COMMAND:command",
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
		}
		if i.OrchestrationId != nil {
			v.OrchestrationID = *i.OrchestrationId
		}
		if i.Path != nil {
			v.Path = *i.Path
		}
		if i.Origin != "" {
			v.Origin = i.Origin
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
