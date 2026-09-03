package commoncmd

import (
	"context"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/client"
)

type (
	CmdDaemonHeartbeatSign struct {
		CmdDaemonSubAction
		Names []string
	}
)

func NewCmdHeartbeatSign() *cobra.Command {
	options := CmdDaemonHeartbeatSign{}
	cmd := &cobra.Command{
		Use:   "sign NAME...",
		Short: "sign a heartbeat disk",
		Long: ForProgram("Write the signature the nodes of a disk heartbeat claim their slot with.\n\n" +
			HeartbeatNameHelp + "\n\nThe heartbeat must be of type disk: this action writes to its dev."),
		Example: ForProgram(`  # sign the disk of hb#2 on the local node
  om daemon hb sign 2

  # sign the disks of hb#2 and hb#3 on every node
  om daemon hb sign 2 3 --node '*'`),
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

func (t *CmdDaemonHeartbeatSign) Run() error {
	return t.CmdDaemonSubAction.RunForEach(t.Names, func(name string) apiFuncWithNode {
		return func(ctx context.Context, c *client.T, nodename string) (response *http.Response, err error) {
			return c.PostDaemonHeartbeatSign(ctx, nodename, name)
		}
	})
}
