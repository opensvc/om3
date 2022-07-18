package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/nodeaction"
	"opensvc.com/opensvc/core/object"
)

type (
	// NodePrintCapabilities is the cobra flag set of the node print capabilities command.
	NodePrintCapabilities struct {
		OptsGlobal
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *NodePrintCapabilities) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
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
		nodeaction.WithFormat(t.Format),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),

		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithRemoteAction("node print capabilities"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"format": t.Format,
		}),

		nodeaction.WithLocal(t.Local),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			n, err := object.NewNode()
			if err != nil {
				return nil, err
			}
			return n.PrintCapabilities()
		}),
	).Do()
}
