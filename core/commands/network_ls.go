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
	"opensvc.com/opensvc/core/rawconfig"
)

type (
	// NetworkLs is the cobra flag set of the command.
	NetworkLs struct {
		Global object.OptsGlobal
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *NetworkLs) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *NetworkLs) cmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "list the cluster networks",
		Run: func(_ *cobra.Command, _ []string) {
			t.run()
		},
	}
}

func (t *NetworkLs) run() {
	var data []string
	if t.Global.Local || !clientcontext.IsSet() {
		data = t.extractLocal()
	} else {
		data = t.extractDaemon()
	}
	output.Renderer{
		Format: t.Global.Format,
		Color:  t.Global.Color,
		Data:   data,
		HumanRenderer: func() string {
			s := ""
			for _, e := range data {
				s += e + "\n"
			}
			return s
		},
		Colorize: rawconfig.Node.Colorize,
	}.Print()
}

func (t *NetworkLs) extractLocal() []string {
	return object.NewNode().ListNetworks()
}

func (t *NetworkLs) extractDaemon() []string {
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
	return []string{}
}
