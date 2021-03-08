package cmd

import (
	"github.com/spf13/cobra"

	"opensvc.com/opensvc/core/entrypoints/monitor"
)

var (
	daemonStatusWatchFlag    bool
	daemonStatusSelectorFlag string
)

var daemonStatusCmd = &cobra.Command{
	Use:     "status",
	Short:   "Print the cluster status",
	Long:    monitor.CmdLong,
	Aliases: []string{"statu"},
	Run:     daemonStatusCmdRun,
}

func init() {
	daemonCmd.AddCommand(daemonStatusCmd)
	daemonStatusCmd.Flags().BoolVarP(&daemonStatusWatchFlag, "watch", "w", false, "Watch the monitor changes")
	daemonStatusCmd.Flags().StringVarP(&daemonStatusSelectorFlag, "selector", "s", "**", "Select opensvc objects (ex: **/db*,*/svc/db*)")
}

func daemonStatusCmdRun(cmd *cobra.Command, args []string) {
	m := monitor.New()
	m.SetWatch(daemonStatusWatchFlag)
	m.SetColor(colorFlag)
	m.SetFormat(formatFlag)
	m.SetServer(serverFlag)
	m.SetSelector(daemonStatusSelectorFlag)
	m.Do()
}
