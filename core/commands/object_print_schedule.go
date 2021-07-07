package commands

import (
	"encoding/json"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/clientcontext"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/output"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/core/schedule"
)

type (
	// CmdObjectPrintSchedule is the cobra flag set of the print schedule command.
	CmdObjectPrintSchedule struct {
		object.OptsPrintSchedule
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectPrintSchedule) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *CmdObjectPrintSchedule) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:     "schedule",
		Short:   "Print selected objects scheduling table",
		Aliases: []string{"schedul", "schedu", "sched", "sche", "sch", "sc"},
		Run: func(cmd *cobra.Command, args []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdObjectPrintSchedule) extract(selector string, c *client.T) schedule.Table {
	if t.Global.Local {
		return t.extractLocal(selector)
	}
	if data, err := t.extractFromDaemon(selector, c); err == nil {
		return data
	}
	if clientcontext.IsSet() {
		log.Error().Msg("can not fetch daemon data")
		return schedule.NewTable()
	}
	return t.extractLocal(selector)
}

func (t *CmdObjectPrintSchedule) extractLocal(selector string) schedule.Table {
	data := schedule.NewTable()
	sel := object.NewSelection(
		selector,
		object.SelectionWithLocal(true),
	)
	type scheduler interface {
		PrintSchedule(object.OptsPrintSchedule) schedule.Table
	}
	for _, p := range sel.Expand() {
		obj := object.NewBaserFromPath(p)
		i, ok := obj.(scheduler)
		if !ok {
			continue
		}
		table := i.PrintSchedule(t.OptsPrintSchedule)
		data = data.Add(table)
	}
	return data
}

func (t *CmdObjectPrintSchedule) extractFromDaemon(selector string, c *client.T) (schedule.Table, error) {
	data := schedule.NewTable()
	req := c.NewGetSchedules()
	req.ObjectSelector = selector
	b, err := req.Do()
	if err != nil {
		return data, err
	}
	err = json.Unmarshal(b, &data)
	if err != nil {
		log.Debug().Err(err).Msg("unmarshal GET /schedules")
		return data, err
	}
	return data, nil
}

func (t *CmdObjectPrintSchedule) run(selector *string, kind string) {
	mergedSelector := mergeSelector(*selector, t.Global.ObjectSelector, kind, "")
	c, err := client.New(client.WithURL(t.Global.Server))
	if err != nil {
		log.Error().Err(err).Msg("")
		os.Exit(1)
	}
	data := t.extract(mergedSelector, c)

	output.Renderer{
		Format:   t.Global.Format,
		Color:    t.Global.Color,
		Data:     data,
		Colorize: rawconfig.Node.Colorize,
		HumanRenderer: func() string {
			return data.Render()
		},
	}.Print()
}
