// Package cmd defines the opensvc command line actions and options.
package cmd

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/check"
)

// nodeEventsCmd represents the nodeEvents command
var nodeChecksCmd = &cobra.Command{
	Use:     "checks",
	Short:   "Run the check drivers, push and print the instances",
	Aliases: []string{"check", "chec", "che", "ch"},
	Run:     nodeChecksCmdRun,
}

func init() {
	nodeCmd.AddCommand(nodeChecksCmd)
}

func nodeChecksCmdRun(cmd *cobra.Command, args []string) {
	check.Runner{
		Color:  colorFlag,
		Format: formatFlag,
	}.Do()
}
