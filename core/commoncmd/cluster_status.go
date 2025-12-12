package commoncmd

import (
	"github.com/opensvc/om3/v3/core/monitor"
	"github.com/spf13/cobra"
)

func NewCmdClusterStatus() *cobra.Command {
	var options CmdObjectMonitor
	cmd := &cobra.Command{
		GroupID: GroupIDQuery,
		Use:     "status",
		Short:   "show the cluster status",
		Long:    monitor.CmdLong,
		Aliases: []string{"statu"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run("**", "")
		},
	}
	flags := cmd.Flags()
	FlagObjectSelector(flags, &options.ObjectSelector)
	FlagColor(flags, &options.Color)
	FlagOutput(flags, &options.Output)
	FlagWatch(flags, &options.Watch)
	FlagOutputSections(flags, &options.Sections)
	return cmd
}

func NewCmdDaemonStatus() *cobra.Command {
	cmd := NewCmdClusterStatus()
	cmd.Hidden = true
	return cmd
}
