package commoncmd

import "github.com/spf13/cobra"

func NewCmdContext() *cobra.Command {
	return &cobra.Command{
		Use:     "context",
		Short:   "manage client contexts",
		Aliases: []string{"ctx"},
	}
}
