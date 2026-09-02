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
		Names []string
	}
)

func NewCmdDaemonHeartbeatStop() *cobra.Command {
	options := CmdDaemonHeartbeatStop{}
	cmd := &cobra.Command{
		Use:   "stop NAME...",
		Short: "stop daemon heartbeat rx or tx streams",
		Long: ForProgram("Stop the named directions of the configured heartbeats.\n\n" +
			HeartbeatStreamNameHelp),
		Example: ForProgram(`  # stop the receiver of hb#1 on the local node
  om daemon hb stop 1.rx

  # stop both streams of hb#2
  om daemon hb stop hb#2

  # stop the sender of hb#2 and the receiver of hb#1 on every node
  om daemon hb stop hb#2.tx 1.rx --node '*'`),
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

func (t *CmdDaemonHeartbeatStop) Run() error {
	return t.CmdDaemonSubAction.RunForEach(t.Names, func(name string) apiFuncWithNode {
		return func(ctx context.Context, c *client.T, nodename string) (response *http.Response, err error) {
			return c.PostDaemonHeartbeatStop(ctx, nodename, name)
		}
	})
}
