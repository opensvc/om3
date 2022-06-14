package commands

import (
	"bufio"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/util/render"
)

type (
	// CmdObjectLogs is the cobra flag set of the logs command.
	CmdObjectLogs struct {
		Global object.OptsGlobal
		Follow bool   `flag:"logs-follow"`
		SID    string `flag:"logs-sid"`
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectLogs) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *CmdObjectLogs) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:     "logs",
		Aliases: []string{"logs", "log", "lo"},
		Short:   "filter and format logs",
		Run: func(cmd *cobra.Command, args []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdObjectLogs) render(fpath string) error {
	f, err := os.Open(fpath)
	if err != nil {
		return err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	w := zerolog.NewConsoleWriter()
	w.TimeFormat = "2006-01-02T15:04:05.000Z07:00"
	w.NoColor = color.NoColor
	for scanner.Scan() {
		b := scanner.Bytes()
		switch t.Global.Format {
		case "json":
			fmt.Printf("%s\n", string(b))
		default:
			_, _ = w.Write(b)
		}
	}
	return nil
}

func (t *CmdObjectLogs) local(selStr string) error {
	sel := object.NewSelection(
		selStr,
		object.SelectionWithLocal(true),
	)
	paths, err := sel.Expand()
	if err != nil {
		return err
	}
	for _, p := range paths {
		fpath := object.LogFile(p)
		if err := t.render(fpath); err != nil {
			return err
		}
	}
	return nil
}

func (t *CmdObjectLogs) run(selector *string, kind string) {
	var err error
	render.SetColor(t.Global.Color)
	mergedSelector := mergeSelector(*selector, t.Global.ObjectSelector, kind, "**")
	if t.Global.Local {
		err = t.local(mergedSelector)
	} else {
		//err = t.remote(mergedSelector)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
