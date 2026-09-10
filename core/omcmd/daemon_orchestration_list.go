package omcmd

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

	params := api.GetOrchestrationsParams{}
	if len(t.States) > 0 {
		params.States = &t.States
	}
	resp, err := c.GetOrchestrationsWithResponse(context.Background(), nodename, &params)
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
	resp, err := c.GetOrchestrationWithResponse(context.Background(), nodename, t.ID)
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
		DefaultOutput: "tab=STATE:state,ID:id,PATH:path,GLOBAL_EXPECT:global_expect,ACCEPTED_BY:node,BEGIN_AT:begin_at",
		Output:        t.Output,
		Color:         t.Color,
		Data:          items,
		Colorize:      rawconfig.Colorize,
	}.Print()
}
