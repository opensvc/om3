package commoncmd

import "github.com/spf13/cobra"

func NewCmdObjectSchedule(kind string) *cobra.Command {
	cmd := &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "schedule",
		Short:   "query job schedule",
		Aliases: []string{"sched"},
	}
	return cmd
}
