package commands

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/fatih/color"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
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

func (t *CmdObjectLogs) parseFile(fpath string) ([][]byte, error) {
	f, err := os.Open(fpath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	var patternSID []byte
	if t.SID != "" {
		patternSID = []byte(fmt.Sprintf("\"sid\":\"%s\"", t.SID))
	}
	events := make([][]byte, 0)
	for scanner.Scan() {
		b := scanner.Bytes()
		if (patternSID != nil) && !bytes.Contains(b, patternSID) {
			continue
		}
		events = append(events, b)
	}
	return events, nil
}

func (t *CmdObjectLogs) gather(paths []path.T) ([][]byte, error) {
	type bareEvent struct {
		TS float64 `json:"t"`
	}
	events := make([][]byte, 0)
	for _, p := range paths {
		fpath := object.LogFile(p)
		if more, err := t.parseFile(fpath); err != nil {
			return events, err
		} else {
			events = append(events, more...)
		}
	}
	sort.Slice(events, func(i, j int) bool {
		var e1, e2 bareEvent
		if err := json.Unmarshal(events[i], &e1); err != nil {
			return false
		}
		if err := json.Unmarshal(events[j], &e2); err != nil {
			return false
		}
		return e1.TS < e2.TS
	})
	return events, nil
}

func (t *CmdObjectLogs) render(events [][]byte) error {
	w := zerolog.NewConsoleWriter()
	w.TimeFormat = "2006-01-02T15:04:05.000Z07:00"
	w.NoColor = color.NoColor
	for _, b := range events {
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
	events, err := t.gather(paths)
	if err != nil {
		return err
	}
	return t.render(events)
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
