package commoncmd

import (
	"context"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/client"
)

type (
	CmdDaemonHeartbeatSign struct {
		CmdDaemonSubAction
		Name string
	}
)

func NewCmdHeartbeatSign() *cobra.Command {
	options := CmdDaemonHeartbeatSign{}
	cmd := &cobra.Command{
		Use:   "sign",
		Short: "sign a heartbeat disk",
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

func (t *CmdDaemonHeartbeatSign) Run() error {
	fn := func(ctx context.Context, c *client.T, nodename string) (response *http.Response, err error) {
		return c.PostDaemonHeartbeatSign(ctx, nodename, t.Name)
	}
	return t.CmdDaemonSubAction.Run(fn)
}
