package cmd

import (
	"github.com/spf13/cobra"
)

var volSelectorFlag string

var volCmd = &cobra.Command{
	Use:   "vol",
	Short: "Manage volumes",
	Long: `A volume is a persistent data provider.
	
A volume is made of disk, fs and sync resources. It is created by a pool,
to satisfy a demand from a volume resource in a service.

Volumes and their subdirectories can be mounted inside containers.

A volume can host cfg and sec keys projections.
`,
}

func init() {
	rootCmd.AddCommand(volCmd)
	volCmd.PersistentFlags().StringVarP(&volSelectorFlag, "selector", "s", "", "The name of the object to select")
}
