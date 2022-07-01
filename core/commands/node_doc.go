package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/nodeaction"
	"opensvc.com/opensvc/core/object"
)

type (
	// NodeDoc is the cobra flag set of the node doc command.
	NodeDoc struct {
		OptsGlobal
		object.OptsDoc
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *NodeDoc) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *NodeDoc) cmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doc",
		Short: "print the documentation of the selected keywords",
		Run: func(_ *cobra.Command, _ []string) {
			t.run()
		},
	}
}

func (t *NodeDoc) run() {
	nodeaction.New(
		nodeaction.WithFormat(t.Format),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),

		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithRemoteAction("node doc"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"kw":     t.Keyword,
			"driver": t.Driver,
		}),

		nodeaction.WithLocal(t.Local),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			return object.NewNode().Doc(t.OptsDoc)
		}),
	).Do()
}
