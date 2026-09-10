package oxcmd

import (
	"context"
	"fmt"
	"net/http"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/api"
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

	params := api.GetSessionsParams{}
	if len(t.States) > 0 {
		params.States = &t.States
	}
	if t.OrchestrationID != "" {
		params.OrchestrationID = &t.OrchestrationID
	}
	resp, err := c.GetSessionsWithResponse(context.Background(), nodename, &params)
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
	resp, err := c.GetSessionWithResponse(context.Background(), nodename, t.ID)
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
		DefaultOutput: "tab=NODE:node,STATE:state,ID:id,PATH:path,ORIGIN:origin,DURATION:duration,COMMAND:command",
		Output:        t.Output,
		Color:         t.Color,
		Data:          items,
		Colorize:      rawconfig.Colorize,
	}.Print()
}
