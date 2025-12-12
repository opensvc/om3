package commoncmd

import (
	"context"
	"fmt"
	"net/http"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/spf13/cobra"
)

type (
	CmdDaemonHeartbeatStop struct {
		CmdDaemonSubAction
		Name string
	}
)

func NewCmdDaemonHeartbeatStop() *cobra.Command {
	options := CmdDaemonHeartbeatStop{}
	cmd := &cobra.Command{
		Use:   "stop",
		Short: fmt.Sprintf("stop a daemon heartbeat rx or tx"),
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

func (t *CmdDaemonHeartbeatStop) Run() error {
	fn := func(ctx context.Context, c *client.T, nodename string) (response *http.Response, err error) {
		return c.PostDaemonHeartbeatStop(ctx, nodename, t.Name)
	}
	return t.CmdDaemonSubAction.Run(fn)
}
