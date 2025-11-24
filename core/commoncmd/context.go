package commoncmd

import "github.com/spf13/cobra"

func NewCmdContext() *cobra.Command {
	return &cobra.Command{
		Use:     "context",
		Short:   "manage client contexts",
		Aliases: []string{"ctx"},
	}
}

func NewCmdContextCluster() *cobra.Command {
	return &cobra.Command{
		Use:   "cluster",
		Short: "manage context cluster endpoints",
	}
}

func NewCmdContextUser() *cobra.Command {
	return &cobra.Command{
		Use:   "user",
		Long:  "A user is a reusable profile to store authentication metadata, preventing property duplication across contexts. Uses the --name as the default login name when the optional --username property is omitted.",
		Short: "manage context users",
	}
}
