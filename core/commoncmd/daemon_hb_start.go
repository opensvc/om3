package commoncmd

import (
	"context"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/client"
)

type (
	CmdDaemonHeartbeatStart struct {
		CmdDaemonSubAction
		Name string
	}
)

func NewCmdDaemonHeartbeatStart() *cobra.Command {
	options := CmdDaemonHeartbeatStart{}
	cmd := &cobra.Command{
		Use:   "start",
		Short: fmt.Sprintf("start a daemon heartbeat rx or tx"),
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

func (t *CmdDaemonHeartbeatStart) Run() error {
	fn := func(ctx context.Context, c *client.T, nodename string) (response *http.Response, err error) {
		return c.PostDaemonHeartbeatStart(ctx, nodename, t.Name)
	}
	return t.CmdDaemonSubAction.Run(fn)
}
