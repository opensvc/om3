package commoncmd

import "github.com/spf13/cobra"

func NewCmdObjectSchedule(kind string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "schedule",
		Short:   "object scheduler commands",
		Aliases: []string{"sched"},
	}
	return cmd
}
