package omcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/api"
)

type (
	CmdDaemonRelayStatus struct {
		OptsGlobal
	}
)

func (t *CmdDaemonRelayStatus) Run() error {
	cli, err := client.New()
	if err != nil {
		return err
	}
	params := api.GetRelayStatusParams{}
	resp, err := cli.GetRelayStatusWithResponse(context.Background(), &params)
	if err != nil {
		return err
	}
	switch resp.StatusCode() {
	case 200:
	case 401:
		return fmt.Errorf("get relay message: %s: %s", resp.JSON401.Title, resp.JSON401.Detail)
	case 403:
		return fmt.Errorf("get relay message: %s: %s", resp.JSON403.Title, resp.JSON403.Detail)
	case 500:
		return fmt.Errorf("get relay message: %s: %s", resp.JSON500.Title, resp.JSON500.Detail)
	default:
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
