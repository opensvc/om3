package cmd

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/monitor"
)

var svcMonitorWatchFlag bool

var svcMonitorCmd = &cobra.Command{
	Use:     "monitor",
	Aliases: []string{"mon", "moni", "monit", "monito"},
	Short:   "Print selected service and instance status summary",
	Long:    monitor.CmdLong,
	Run:     svcMonitorCmdRun,
}

func init() {
	svcCmd.AddCommand(svcMonitorCmd)
	svcMonitorCmd.Flags().BoolVarP(&svcMonitorWatchFlag, "watch", "w", false, "Watch the monitor changes")
}

func svcMonitorCmdRun(cmd *cobra.Command, args []string) {
	selector := mergeSelector(svcSelectorFlag, "svc", "")
	m := monitor.New()
	m.SetWatch(svcMonitorWatchFlag)
	m.SetColor(colorFlag)
	m.SetFormat(formatFlag)
	m.SetServer(serverFlag)
	m.SetSelector(selector)
	m.SetSections([]string{"objects"})
	m.Do()
}
