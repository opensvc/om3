package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/nodeaction"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
)

type (
	// NodePrintCapabilities is the cobra flag set of the node print capabilities command.
	NodePrintCapabilities struct {
		Global object.OptsGlobal
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *NodePrintCapabilities) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, &t.Global)
}

func (t *NodePrintCapabilities) cmd() *cobra.Command {
	return &cobra.Command{
		Use:     "capabilities",
		Short:   "print the node capabilities",
		Aliases: []string{"capa", "cap", "ca", "caps"},
		Run: func(_ *cobra.Command, _ []string) {
			t.run()
		},
	}
}

func (t *NodePrintCapabilities) run() {
	nodeaction.New(
		nodeaction.WithFormat(t.Global.Format),
		nodeaction.WithColor(t.Global.Color),
		nodeaction.WithServer(t.Global.Server),

		nodeaction.WithRemoteNodes(t.Global.NodeSelector),
		nodeaction.WithRemoteAction("node print capabilities"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"format": t.Global.Format,
		}),

		nodeaction.WithLocal(t.Global.Local),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			return object.NewNode().PrintCapabilities()
		}),
	).Do()
}
