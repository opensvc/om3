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
		Short: "manage cluster contexts",
	}
}

func NewCmdContextUser() *cobra.Command {
	return &cobra.Command{
		Use:   "user",
		Short: "manage user contexts",
	}
}
