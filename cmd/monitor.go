package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/event"
	"opensvc.com/opensvc/core/monitor"
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
	RunE:    monCmdRun,
}

func init() {
	root.AddCommand(monCmd)
	monCmd.Flags().StringVarP(&monSelectorFlag, "selector", "s", "*", "An object selector expression")
	monCmd.Flags().BoolVarP(&monWatchFlag, "watch", "w", false, "Watch the monitor changes")
}

func monCmdRun(_ *cobra.Command, _ []string) error {
	// TODO move to command
	m := monitor.New()
	m.SetColor(colorFlag)
	m.SetFormat(formatFlag)
	cli, err := newClient()
	if err != nil {
		return err
	}
	if monWatchFlag {
		maxRetries := 5
		retries := 0
		evReader, statusGetter, err := monCmdWatchArgs(cli)
		if err != nil {
			return err
		}
		for {
			err := m.DoWatch(statusGetter, evReader, os.Stdout)
			if err1 := evReader.Close(); err1 != nil {
				return fmt.Errorf("monitor watch error '%s' + close event reader error '%s'", err, err1)
			}
			if err == nil {
				return nil
			}
			for {
				retries++
				if retries > maxRetries {
					return err
				} else if retries == 1 {
					_, _ = fmt.Fprintf(os.Stderr, "monitor watch error '%s'\n", err)
					_, _ = fmt.Fprintln(os.Stderr, "press ctrl+c to interrupt retries")
				}
				time.Sleep(time.Second)
				evReader, statusGetter, err = monCmdWatchArgs(cli)
				if err == nil {
					retries = 0
					break
				}
				_, _ = fmt.Fprintf(os.Stderr, "retry %d/%d %s...\n", retries, maxRetries, err)
			}
		}
	} else {
		getter := cli.NewGetDaemonStatus().SetSelector(monSelectorFlag)
		return m.Do(getter, os.Stdout)
	}
}

// monCmdWatchArgs returns evReader and statusGetter for monitor DoWatch
func monCmdWatchArgs(cli *client.T) (evReader event.ReadCloser, statusGetter monitor.Getter, err error) {
	//var reqCli api.GetEventReader
	evReader, err = cli.NewGetEvents().SetSelector(monSelectorFlag).GetReader()
	if err != nil {
		return
	}
	statusGetter = cli.NewGetDaemonStatus().SetSelector(monSelectorFlag)
	return
}
