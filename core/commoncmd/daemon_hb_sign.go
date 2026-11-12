package commoncmd

import (
	"context"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	CmdDaemonHeartbeatSign struct {
		CmdDaemonSubAction
		Names []string
		Force bool
	}
)

func NewCmdHeartbeatSign() *cobra.Command {
	options := CmdDaemonHeartbeatSign{}
	cmd := &cobra.Command{
		Use:   "sign NAME...",
		Short: "sign a heartbeat disk",
		Long: ForProgram("Write the signature the nodes of a disk heartbeat claim their slot with.\n\n" +
			HeartbeatNameHelp + "\n\nThe heartbeat must be of type disk: this action writes to its dev.\n\n" +
			"A disk with no heartbeat signature gets its metadata area, max_slots pages of 4 KiB, zeroed first, so its nodes start from free slots. " +
			"One holding another signature, as a filesystem or a lvm physical volume, is refused, as the sign would erase it: --force signs it anyway. " +
			"A disk already signed keeps the slots of its nodes, and of the other clusters sharing it.\n\n" +
			"A disk too small for the metadata area and a data slot of 1 MiB per node, after an unused one, is refused.\n\n" +
			"The streams of the heartbeat suspended on a disk not signed, on every node, resume within an interval, with no restart."),
		Example: ForProgram(`  # sign the disk of hb#2 on the local node
  om daemon hb sign 2

  # sign the disks of hb#2 and hb#3 on every node
  om daemon hb sign 2 3 --node '*'

  # sign the disk of hb#2, which held a filesystem
  om daemon hb sign 2 --force`),
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: validHeartbeatNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			options.Names = args
			return options.Run()
		},
	}
	flags := cmd.Flags()
	FlagNodeSelector(flags, &options.NodeSelector)
	flags.BoolVar(&options.Force, "force", false, "sign a disk holding another signature, as a filesystem, which the sign erases")
	return cmd
}

func (t *CmdDaemonHeartbeatSign) Run() error {
	return t.CmdDaemonSubAction.RunForEach(t.Names, func(name string) apiFuncWithNode {
		return func(ctx context.Context, c *client.T, nodename string) (response *http.Response, err error) {
			return c.PostDaemonHeartbeatSign(ctx, nodename, name, &api.PostDaemonHeartbeatSignParams{Force: &t.Force})
		}
	})
}
