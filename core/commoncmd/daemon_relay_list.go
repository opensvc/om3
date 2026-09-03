package commoncmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	CmdDaemonRelayList struct {
		Color  string
		Output string
	}
)

func NewCmdDaemonRelayList() *cobra.Command {
	var options CmdDaemonRelayList
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "list the local daemon relay clients and their last data update time",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	FlagColor(flags, &options.Color)
	FlagOutput(flags, &options.Output)
	return cmd
}

func (t *CmdDaemonRelayList) Run() error {
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

// NewCmdDaemonRelayStatus is the name the list command answered to before
// the listings were named after what they render: a row per relay client.
// It is kept for the readers whose fingers and scripts type it.
func NewCmdDaemonRelayStatus() *cobra.Command {
	cmd := NewCmdDaemonRelayList()
	cmd.Use = "status"
	cmd.Aliases = nil
	cmd.Hidden = true
	return cmd
}
