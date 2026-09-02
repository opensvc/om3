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
		Names []string
	}
)

func NewCmdDaemonHeartbeatStart() *cobra.Command {
	options := CmdDaemonHeartbeatStart{}
	cmd := &cobra.Command{
		Use:   "start NAME...",
		Short: "start daemon heartbeat rx or tx streams",
		Long: ForProgram("Start the named directions of the configured heartbeats.\n\n" +
			HeartbeatStreamNameHelp),
		Example: ForProgram(`  # start the receiver of hb#1 on the local node
  om daemon hb start 1.rx

  # start both streams of hb#1
  om daemon hb start 1

  # start the sender of hb#1 and the receiver of hb#2 on every node
  om daemon hb start 1.tx 2.rx --node '*'

  # the id "om daemon hb ls" shows is accepted as it reads
  om daemon hb start hb#1.rx`),
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: validHeartbeatStreamNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			options.Names = args
			return options.Run()
		},
	}
	flags := cmd.Flags()
	FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func (t *CmdDaemonHeartbeatStart) Run() error {
	return t.CmdDaemonSubAction.RunForEach(t.Names, func(name string) apiFuncWithNode {
		return func(ctx context.Context, c *client.T, nodename string) (response *http.Response, err error) {
			return c.PostDaemonHeartbeatStart(ctx, nodename, name)
		}
	})
}
