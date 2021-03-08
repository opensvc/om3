package cmd

import (
	"github.com/spf13/cobra"
)

var svcSelectorFlag string

var svcCmd = &cobra.Command{
	Use:   "svc",
	Short: "Manage services",
	Long: `Service objects subsystem.
	
A service is typically made of ip, app, container and task resources.

They can use support objects like volumes, secrets and configmaps to
isolate lifecycles or to abstract cluster-specific knowledge.
`,
}

func init() {
	rootCmd.AddCommand(svcCmd)
	svcCmd.PersistentFlags().StringVarP(&svcSelectorFlag, "selector", "s", "", "The name of the object to select")
}
