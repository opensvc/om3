package commoncmd

import (
	"context"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/core/client"
)

type (
	CmdDaemonHeartbeatWipe struct {
		CmdDaemonSubAction
		Name string
	}
)

func NewCmdHeartbeatWipe() *cobra.Command {
	options := CmdDaemonHeartbeatWipe{}
	cmd := &cobra.Command{
		Use:   "wipe",
		Short: "wipe a heartbeat disk",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	FlagNodeSelector(flags, &options.NodeSelector)
	FlagDaemonHeartbeatName(flags, &options.Name)
	cmd.MarkFlagRequired("name")
	return cmd
}

func (t *CmdDaemonHeartbeatWipe) Run() error {
	fn := func(ctx context.Context, c *client.T, nodename string) (response *http.Response, err error) {
		return c.PostDaemonHeartbeatWipe(ctx, nodename, t.Name)
	}
	return t.CmdDaemonSubAction.Run(fn)
}
