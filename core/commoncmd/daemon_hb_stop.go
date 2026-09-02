package commoncmd

import (
	"context"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/client"
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
		Use:   "stop NAME",
		Short: fmt.Sprintf("stop a daemon heartbeat rx or tx"),
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

func (t *CmdDaemonHeartbeatStop) Run() error {
	fn := func(ctx context.Context, c *client.T, nodename string) (response *http.Response, err error) {
		return c.PostDaemonHeartbeatStop(ctx, nodename, t.Name)
	}
	return t.CmdDaemonSubAction.Run(fn)
}
