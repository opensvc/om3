package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/nodeaction"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
)

type (
	// NodeUnset is the cobra flag set of the start command.
	NodeUnset struct {
		object.OptsUnset
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *NodeUnset) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *NodeUnset) cmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unset",
		Short: "unset a configuration key",
		Run: func(_ *cobra.Command, _ []string) {
			t.run()
		},
	}
}

func (t *NodeUnset) run() {
	nodeaction.New(
		nodeaction.LocalFirst(),
		nodeaction.WithLocal(t.Local),
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Format),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),
		nodeaction.WithRemoteAction("unset"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"kw": t.Keywords,
		}),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			return nil, object.NewNode().Unset(t.OptsUnset)
		}),
	).Do()
}
