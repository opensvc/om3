package commands

import (
	"encoding/json"
	"fmt"
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
	// NodePrintSchedule is the cobra flag set of the print schedule command.
	NodePrintSchedule struct {
		OptsGlobal
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *NodePrintSchedule) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *NodePrintSchedule) cmd() *cobra.Command {
	return &cobra.Command{
		Use:     "schedule",
		Short:   "Print selected objects scheduling table",
		Aliases: []string{"schedul", "schedu", "sched", "sche", "sch", "sc"},
		Run: func(cmd *cobra.Command, args []string) {
			t.run()
		},
	}
}

func (t *NodePrintSchedule) extract(c *client.T) schedule.Table {
	if t.Local {
		return t.extractLocal()
	}
	if data, err := t.extractFromDaemon(c); err == nil {
		return data
	}
	if clientcontext.IsSet() {
		log.Error().Msg("can not fetch daemon data")
		return schedule.NewTable()
	}
	return t.extractLocal()
}

func (t *NodePrintSchedule) extractLocal() schedule.Table {
	data := schedule.NewTable()
	obj, err := object.NewNode()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", err)
		os.Exit(1)
	}
	table := obj.PrintSchedule()
	data = data.Add(table)
	return data
}

func (t *NodePrintSchedule) extractFromDaemon(c *client.T) (schedule.Table, error) {
	data := schedule.NewTable()
	req := c.NewGetSchedules()
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

func (t *NodePrintSchedule) run() {
	c, err := client.New(client.WithURL(t.Server))
	if err != nil {
		log.Error().Err(err).Msg("")
		os.Exit(1)
	}
	data := t.extract(c)

	output.Renderer{
		Format:   t.Format,
		Color:    t.Color,
		Data:     data,
		Colorize: rawconfig.Colorize,
		HumanRenderer: func() string {
			return data.Render()
		},
	}.Print()
}
