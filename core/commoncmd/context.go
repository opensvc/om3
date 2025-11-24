package commoncmd

import "github.com/spf13/cobra"

func NewCmdContext() *cobra.Command {
	return &cobra.Command{
		Use: "context",
		Long: `A context groups the namespace, authentication and endpoint metadata needed to connect and manage a remote cluster.

Once configured, you can login and logout from this context.`,
		Short:   "manage client contexts",
		Aliases: []string{"ctx"},
	}
}

func NewCmdContextCluster() *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "cluster",
		Long:    "A cluster is a reusable profile to store communication metadata, preventing property duplication across contexts.",
		Short:   "manage context cluster endpoints",
	}
}

func NewCmdContextUser() *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "user",
		Long:    "A user is a reusable profile to store authentication metadata, preventing property duplication across contexts.\nUses the --name as the default login name when the optional --username property is omitted.",
		Short:   "manage context user authentication",
	}
}
