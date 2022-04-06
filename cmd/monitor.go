package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"opensvc.com/opensvc/core/client"
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
	root.AddCommand(monCmd)
	monCmd.Flags().StringVarP(&monSelectorFlag, "selector", "s", "*", "An object selector expression")
	monCmd.Flags().BoolVarP(&monWatchFlag, "watch", "w", false, "Watch the monitor changes")
}

func monCmdRun(_ *cobra.Command, _ []string) {
	m := monitor.New()
	m.SetColor(colorFlag)
	m.SetFormat(formatFlag)
	cli, err := client.New(client.WithURL(serverFlag))
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return
	}
	if monWatchFlag {
		getter := cli.NewGetEvents().SetSelector(monSelectorFlag)
		if err = m.DoWatch(getter, os.Stdout); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			return
		}
	} else {
		getter := cli.NewGetDaemonStatus().SetSelector(monSelectorFlag)
		m.Do(getter, os.Stdout)
	}
}

// TODO suppress when replaced monCmdRun
var monDemoCmd = &cobra.Command{
	Use:   "mondemo",
	Short: "Print the cluster status",
	Long:  monitor.CmdLong,
	Run:   monDemoCmdRun,
}

func init() {
	root.AddCommand(monDemoCmd)
	monDemoCmd.Flags().StringVarP(&monSelectorFlag, "selector", "s", "*", "An object selector expression")
	monDemoCmd.Flags().BoolVarP(&monWatchFlag, "watch", "w", false, "Watch the monitor changes")
}

func monDemoCmdRun(_ *cobra.Command, _ []string) {
	// TODO suppress when replaced monCmdRun
	m := monitor.New()
	m.SetColor(colorFlag)
	m.SetFormat(formatFlag)
	cli, err := newClient()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return
	}
	if monWatchFlag {
		evGetter := cli.NewGetEventsDemo()
		statusGetter := cli.NewGetDaemonStatus().SetSelector(daemonStatusSelectorFlag)
		if err = m.DoWatchDemo(statusGetter, evGetter, os.Stdout); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			return
		}
	} else {
		getter := cli.NewGetDaemonStatus().SetSelector(monSelectorFlag)
		m.Do(getter, os.Stdout)
	}
}
