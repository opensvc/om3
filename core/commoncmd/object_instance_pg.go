package commoncmd

import "github.com/spf13/cobra"

func NewCmdObjectInstancePG(kind string) *cobra.Command {
	cmd := &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "pg",
		Short:   "manage instance process group settings",
	}
	cmd.AddGroup(
		NewGroupSubsystems(),
	)
	return cmd
}
