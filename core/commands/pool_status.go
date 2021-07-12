package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/clientcontext"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/output"
	"opensvc.com/opensvc/core/pool"
	"opensvc.com/opensvc/core/rawconfig"
)

type (
	// PoolStatus is the cobra flag set of the command.
	PoolStatus struct {
		Global object.OptsGlobal
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *PoolStatus) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *PoolStatus) cmd() *cobra.Command {
	return &cobra.Command{
		Use:     "status",
		Short:   "show the cluster pools usage",
		Aliases: []string{"statu", "stat", "sta", "st"},
		Run: func(_ *cobra.Command, _ []string) {
			t.run()
		},
	}
}

func (t *PoolStatus) run() {
	var data pool.StatusList
	if t.Global.Local || !clientcontext.IsSet() {
		data = t.extractLocal()
	} else {
		data = t.extractDaemon()
	}
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

func (t *PoolStatus) extractLocal() pool.StatusList {
	return object.NewNode().ShowPools()
}

func (t *PoolStatus) extractDaemon() pool.StatusList {
	var (
		c   *client.T
		err error
	)
	if c, err = client.New(client.WithURL(t.Global.Server)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	panic("TODO")
	fmt.Println(c)
	return pool.NewStatusList()
}
