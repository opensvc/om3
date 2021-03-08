package cmd

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/monitor"
)

var (
	monWatchFlag    bool
	monSelectorFlag string
)

var monCmd = &cobra.Command{
	Use:     "monitor",
	Aliases: []string{"m", "mo", "mon", "moni", "monit", "monito"},
	Short:   "Print the cluster status",
	Long:    monitor.CmdLong,
	Run:     monCmdRun,
}

func init() {
	rootCmd.AddCommand(monCmd)
	monCmd.Flags().StringVarP(&monSelectorFlag, "selector", "s", "*", "An object selector expression")
	monCmd.Flags().BoolVarP(&monWatchFlag, "watch", "w", false, "Watch the monitor changes")
}

func monCmdRun(cmd *cobra.Command, args []string) {
	m := monitor.New()
	m.SetWatch(monWatchFlag)
	m.SetColor(colorFlag)
	m.SetFormat(formatFlag)
	m.SetServer(serverFlag)
	m.SetSelector(monSelectorFlag)
	m.Do()
}
