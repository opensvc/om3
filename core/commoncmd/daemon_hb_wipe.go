package commoncmd

import (
	"context"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/client"
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
		Use:   "wipe NAME",
		Short: "wipe a heartbeat disk",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			options.Name = args[0]
			return options.Run()
		},
	}
	flags := cmd.Flags()
	FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func (t *CmdDaemonHeartbeatWipe) Run() error {
	fn := func(ctx context.Context, c *client.T, nodename string) (response *http.Response, err error) {
		return c.PostDaemonHeartbeatWipe(ctx, nodename, t.Name)
	}
	return t.CmdDaemonSubAction.Run(fn)
}
