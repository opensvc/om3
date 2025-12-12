package commoncmd

import (
	"context"
	"fmt"
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
		Use:   "restart",
		Short: fmt.Sprintf("restart daemon heartbeat component `name`"),
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

func (t *CmdDaemonHeartbeatRestart) Run() error {
	fn := func(ctx context.Context, c *client.T, nodename string) (response *http.Response, err error) {
		return c.PostDaemonHeartbeatRestart(ctx, nodename, t.Name)
	}
	return t.CmdDaemonSubAction.Run(fn)
}
