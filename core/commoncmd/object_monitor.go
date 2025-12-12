package commoncmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/monitor"
)

type (
	CmdObjectMonitor struct {
		Color          string
		ObjectSelector string
		Output         string
		Sections       string
		Watch          bool
	}
)

func NewCmdMonitor() *cobra.Command {
	return NewCmdObjectMonitor("*", "")
}

func NewCmdObjectMonitor(selector, kind string) *cobra.Command {
	var options CmdObjectMonitor
	cmd := &cobra.Command{
		GroupID: GroupIDQuery,
		Use:     "monitor",
		Aliases: []string{"m", "mo", "mon", "moni", "monit", "monito"},
		Short:   "show the cluster status",
		Long:    monitor.CmdLong,
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(selector, kind)
		},
	}
	flags := cmd.Flags()
	FlagColor(flags, &options.Color)
	FlagObjectSelector(flags, &options.ObjectSelector)
	FlagOutput(flags, &options.Output)
	FlagOutputSections(flags, &options.Sections)
	FlagWatch(flags, &options.Watch)
	return cmd
}

func (t *CmdObjectMonitor) Run(selector, kind string) error {
	defaultSelector := ""
	if kind != "" {
		defaultSelector = fmt.Sprintf("*/%s/*", kind)
	}
	mergedSelector := MergeSelector(selector, t.ObjectSelector, kind, defaultSelector)

	cli, err := client.New(client.WithTimeout(0))
	if err != nil {
		return err
	}

	m := monitor.New()
	m.SetColor(t.Color)
	m.SetFormat(t.Output)
	m.SetSectionsFromExpression(t.Sections)
	m.SetSelector(mergedSelector)

	if t.Watch {
		maxRetries := 600
		retries := 0
		evReader, err := cli.NewGetEvents().SetSelector(mergedSelector).GetReader()
		if err != nil {
			return err
		}
		for {
			statusGetter := cli.NewGetClusterStatus().SetSelector(mergedSelector)
			err := m.DoWatch(statusGetter, evReader, os.Stdout)
			if err1 := evReader.Close(); err1 != nil {
				return fmt.Errorf("object monitor watch error '%s' + close event reader error '%s'", err, err1)
			}
			if err == nil {
				return err
			}
			for {
				retries++
				if retries > maxRetries {
					return err
				} else if retries == 1 {
					_, _ = fmt.Fprintf(os.Stderr, "object monitor watch error '%s'\n", err)
					_, _ = fmt.Fprintln(os.Stderr, "press ctrl+c to interrupt retries")
				}
				time.Sleep(time.Second)
				evReader, err = cli.NewGetEvents().SetSelector(mergedSelector).GetReader()
				if err == nil {
					retries = 0
					break
				}
				_, _ = fmt.Fprintf(os.Stderr, "retry %d/%d %s...\n", retries, maxRetries, err)
			}
		}
	} else {
		getter := cli.NewGetClusterStatus().SetSelector(mergedSelector)
		if err := m.Do(getter, os.Stdout); err != nil {
			return err
		}
	}
	return nil
}
