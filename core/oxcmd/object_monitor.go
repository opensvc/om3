package oxcmd

import (
	"fmt"
	"os"
	"time"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/monitor"
)

type (
	CmdObjectMonitor struct {
		OptsGlobal
		Watch    bool
		Sections string
	}
)

func (t *CmdObjectMonitor) Run(selector, kind string) error {
	defaultSelector := ""
	if kind != "" {
		defaultSelector = fmt.Sprintf("*/%s/*", kind)
	}
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, defaultSelector)

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
