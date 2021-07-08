package cmd

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/commands"
)

var (
	poolCmd = &cobra.Command{
		Use:   "pool",
		Short: "Manage storage pools",
		Long:  ` A pool is a vol provider. Pools abstract the hardware and software specificities of the cluster infrastructure.`,
	}
	poolCreateCmd = &cobra.Command{
		Use:     "create",
		Short:   "create a pool object",
		Aliases: []string{"creat", "crea", "cre", "cr"},
	}
)

func init() {
	var (
		cmdPoolLs commands.PoolLs
	)
	rootCmd.AddCommand(poolCmd)
	poolCmd.AddCommand(poolCreateCmd)

	cmdPoolLs.Init(poolCmd)
}
