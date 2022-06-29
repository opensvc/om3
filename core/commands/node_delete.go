package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/nodeaction"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
)

type (
	// NodeDelete is the cobra flag set of the delete command.
	NodeDelete struct {
		OptsGlobal
		object.OptsDelete
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *NodeDelete) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *NodeDelete) cmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete",
		Short: "delete a configuration section",
		Run: func(_ *cobra.Command, _ []string) {
			t.run()
		},
	}
}

func (t *NodeDelete) run() {
	nodeaction.New(
		nodeaction.LocalFirst(),
		nodeaction.WithLocal(t.Local),
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Format),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),
		nodeaction.WithRemoteAction("delete"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"rid":         t.OptsDelete.RID,
			"unprovision": t.OptsDelete.Unprovision,
		}),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			return nil, object.NewNode().Delete(t.OptsDelete)
		}),
	).Do()
}
