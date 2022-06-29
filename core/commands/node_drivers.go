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
		OptsGlobal
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *NodeDrivers) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
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
		nodeaction.WithFormat(t.Format),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),

		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithRemoteAction("node drivers"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"format": t.Format,
		}),

		nodeaction.WithLocal(t.Local),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			return object.NewNode().Drivers()
		}),
	).Do()
}
