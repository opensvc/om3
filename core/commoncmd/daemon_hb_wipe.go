package commoncmd

import (
	"context"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/client"
)

type (
	CmdDaemonHeartbeatWipe struct {
		CmdDaemonSubAction
		Names []string
	}
)

func NewCmdHeartbeatWipe() *cobra.Command {
	options := CmdDaemonHeartbeatWipe{}
	cmd := &cobra.Command{
		Use:   "wipe NAME...",
		Short: "wipe a heartbeat disk",
		Long: ForProgram("Remove the signature the nodes of a disk heartbeat claim their slot with.\n\n" +
			HeartbeatNameHelp + "\n\nThe heartbeat must be of type disk: this action writes to its dev."),
		Example: ForProgram(`  # wipe the disk of hb#2 on the local node
  om daemon hb wipe 2

  # wipe the disks of hb#2 and hb#3 on every node
  om daemon hb wipe 2 3 --node '*'`),
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: validHeartbeatNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			options.Names = args
			return options.Run()
		},
	}
	flags := cmd.Flags()
	FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func (t *CmdDaemonHeartbeatWipe) Run() error {
	return t.CmdDaemonSubAction.RunForEach(t.Names, func(name string) apiFuncWithNode {
		return func(ctx context.Context, c *client.T, nodename string) (response *http.Response, err error) {
			return c.PostDaemonHeartbeatWipe(ctx, nodename, name)
		}
	})
}
