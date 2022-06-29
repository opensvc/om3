package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/nodeselector"
	"opensvc.com/opensvc/core/output"
	"opensvc.com/opensvc/core/rawconfig"
)

type (
	// NodeLs is the cobra flag set of the command.
	NodeLs struct {
		OptsGlobal
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *NodeLs) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *NodeLs) cmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "list the cluster nodes",
		Long:  "The list can be filtered using the --node selector. This command can be used to validate node selector expressions.",
		Run: func(_ *cobra.Command, _ []string) {
			t.run()
		},
	}
}

func (t *NodeLs) run() {
	var (
		c        *client.T
		err      error
		selector string
	)
	if !t.Local {
		if c, err = client.New(client.WithURL(t.Server)); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	if t.NodeSelector == "" {
		selector = "*"
	} else {
		selector = t.NodeSelector
	}
	nodes := nodeselector.New(
		selector,
		nodeselector.WithLocal(t.Local),
		nodeselector.WithServer(t.Server),
		nodeselector.WithClient(c),
	).Expand()
	output.Renderer{
		Format: t.Format,
		Color:  t.Color,
		Data:   nodes,
		HumanRenderer: func() string {
			s := ""
			for _, e := range nodes {
				s += e + "\n"
			}
			return s
		},
		Colorize: rawconfig.Colorize,
	}.Print()
}
