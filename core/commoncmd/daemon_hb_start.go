package commoncmd

import (
	"context"
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
		Use:   "start NAME",
		Short: "start a daemon heartbeat rx or tx",
		Long: ForProgram("Start one direction of a configured heartbeat.\n\n" +
			HeartbeatStreamNameHelp),
		Example: ForProgram(`  # start the receiver of hb#1 on the local node
  om daemon hb start 1.rx

  # start the sender of hb#1 on every node
  om daemon hb start 1.tx --node '*'

  # the id "om daemon hb status" shows is accepted as it reads
  om daemon hb start hb#1.rx`),
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

func (t *CmdDaemonHeartbeatStart) Run() error {
	fn := func(ctx context.Context, c *client.T, nodename string) (response *http.Response, err error) {
		return c.PostDaemonHeartbeatStart(ctx, nodename, t.Name)
	}
	return t.CmdDaemonSubAction.Run(fn)
}
