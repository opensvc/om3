package oxcmd

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/api"
)

type (
	CmdNodeRelayStatus struct {
		OptsGlobal
		Relays string
	}
)

func (t *CmdNodeRelayStatus) Run() error {
	cli, err := client.New()
	if err != nil {
		return err
	}
	v := true
	params := api.GetRelayStatusParams{
		Remote: &v,
	}
	if t.Relays != "" {
		relays := strings.Split(t.Relays, ",")
		params.Relays = &relays
	}
	resp, err := cli.GetRelayStatusWithResponse(context.Background(), &params)
	if err != nil {
		return err
	} else if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("unexpected get relay message status code %s", resp.Status())
	}
	output.Renderer{
		DefaultOutput: "tab=RELAY:relay,USERNAME:username,CLUSTER_ID:cluster_id,CLUSTER_NAME:cluster_name,NODENAME:nodename,NODE_ADDR:node_addr,UPDATED_AT:updated_at,MSG_LEN:msg_len",
		Output:        t.Output,
		Color:         t.Color,
		Data:          *resp.JSON200,
		Colorize:      rawconfig.Colorize,
	}.Print()
	return nil
}
