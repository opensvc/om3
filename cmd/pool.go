package cmd

import (
	"github.com/spf13/cobra"
)

var (
	cmdPool = &cobra.Command{
		Use:   "pool",
		Short: "Manage storage pools",
		Long:  ` A pool is a vol provider. Pools abstract the hardware and software specificities of the cluster infrastructure.`,
	}
	cmdPoolCreate = &cobra.Command{
		Use:     "create",
		Short:   "create a pool object",
		Aliases: []string{"creat", "crea", "cre", "cr"},
	}
)

func init() {
	root.AddCommand(
		cmdPool,
	)
	cmdPool.AddCommand(
		cmdPoolCreate,
		newCmdPoolLs(),
		newCmdPoolStatus(),
	)
}
