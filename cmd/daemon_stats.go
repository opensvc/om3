package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints"
)

var daemonStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Print the resource usage statistics.",
	Run:   daemonStatsCmdRun,
}

func init() {
	daemonCmd.AddCommand(daemonStatsCmd)
}

func daemonStatsCmdRun(cmd *cobra.Command, args []string) {
	err := entrypoints.DaemonStats{
		Format: formatFlag,
		Color:  colorFlag,
		Server: serverFlag,
	}.Do()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
