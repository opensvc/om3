package commoncmd

import "github.com/spf13/cobra"

func NewCmdNode() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Short: "manage a opensvc cluster node",
	}
	cmd.AddGroup(
		NewGroupOrchestratedActions(),
		NewGroupQuery(),
	)
	return cmd
}
