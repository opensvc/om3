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
		Names []string
	}
)

func NewCmdDaemonHeartbeatRestart() *cobra.Command {
	options := CmdDaemonHeartbeatRestart{}
	cmd := &cobra.Command{
		Use:   "restart NAME...",
		Short: "restart daemon heartbeat rx or tx streams",
		Long: ForProgram("Stop then start the named directions of the configured heartbeats.\n\n" +
			HeartbeatStreamNameHelp),
		Example: ForProgram(`  # restart the receiver of hb#1 on the local node
  om daemon hb restart 1.rx

  # restart both streams of hb#1
  om daemon hb restart 1

  # restart both streams of hb#1 and hb#2 on every node
  om daemon hb restart 1 2 --node '*'`),
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

func (t *CmdDaemonHeartbeatRestart) Run() error {
	return t.CmdDaemonSubAction.RunForEach(t.Names, func(name string) apiFuncWithNode {
		return func(ctx context.Context, c *client.T, nodename string) (response *http.Response, err error) {
			return c.PostDaemonHeartbeatRestart(ctx, nodename, name)
		}
	})
}
