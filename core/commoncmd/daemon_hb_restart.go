package commoncmd

import (
	"context"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/client"
)

type (
	CmdDaemonHeartbeatRestart struct {
		CmdDaemonSubAction
		Name string
	}
)

func NewCmdDaemonHeartbeatRestart() *cobra.Command {
	options := CmdDaemonHeartbeatRestart{}
	cmd := &cobra.Command{
		Use:   "restart NAME",
		Short: "restart a daemon heartbeat rx or tx",
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

func (t *CmdDaemonHeartbeatRestart) Run() error {
	fn := func(ctx context.Context, c *client.T, nodename string) (response *http.Response, err error) {
		return c.PostDaemonHeartbeatRestart(ctx, nodename, t.Name)
	}
	return t.CmdDaemonSubAction.Run(fn)
}
