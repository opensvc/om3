package ox

import (
	"github.com/spf13/cobra"
)

var (
	cmdPool = &cobra.Command{
		Use:   "pool",
		Short: "manage storage pools",
		Long:  ` A pool is a vol provider. Pools abstract the hardware and software specificities of the cluster infrastructure.`,
	}
	cmdPoolVolume = &cobra.Command{
		Use:     "volume",
		Short:   "manage storage pool volumes",
		Aliases: []string{"vol"},
	}
)

func init() {
	root.AddCommand(
		cmdPool,
	)
	cmdPool.AddCommand(
		cmdPoolVolume,
		newCmdPoolLs(),
	)
	cmdPoolVolume.AddCommand(
		newCmdPoolVolumeLs(),
	)
}
