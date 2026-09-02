package commoncmd

import (
	"context"
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
		Short: "stop a daemon heartbeat rx or tx",
		Long: ForProgram("Stop one direction of a configured heartbeat.\n\n" +
			HeartbeatStreamNameHelp),
		Example: ForProgram(`  # stop the receiver of hb#1 on the local node
  om daemon hb stop 1.rx

  # stop the sender of hb#1 on every node
  om daemon hb stop 1.tx --node '*'`),
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: validHeartbeatStreamNames,
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
