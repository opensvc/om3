package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/nodeaction"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
)

type (
	// NodeDrivers is the cobra flag set of the node drivers command.
	NodeDrivers struct {
		Global object.OptsGlobal
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *NodeDrivers) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, &t.Global)
}

func (t *NodeDrivers) cmd() *cobra.Command {
	return &cobra.Command{
		Use:     "drivers",
		Short:   "list builtin drivers",
		Aliases: []string{"driver", "drive", "driv", "drv", "dr"},
		Run: func(_ *cobra.Command, _ []string) {
			t.run()
		},
	}
}

func (t *NodeDrivers) run() {
	nodeaction.New(
		nodeaction.WithFormat(t.Global.Format),
		nodeaction.WithColor(t.Global.Color),
		nodeaction.WithServer(t.Global.Server),

		nodeaction.WithRemoteNodes(t.Global.NodeSelector),
		nodeaction.WithRemoteAction("node drivers"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"format": t.Global.Format,
		}),

		nodeaction.WithLocal(t.Global.Local),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			return object.NewNode().Drivers()
		}),
	).Do()
}
