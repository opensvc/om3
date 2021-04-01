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

func daemonStatsCmdRun(_ *cobra.Command, _ []string) {
	err := entrypoints.DaemonStats{
		Format: formatFlag,
		Color:  colorFlag,
		Server: serverFlag,
	}.Do()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
