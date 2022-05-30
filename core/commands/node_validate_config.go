package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/nodeaction"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
)

type (
	// NodeValidateConfig is the cobra flag set of the start command.
	NodeValidateConfig struct {
		object.OptsValidateConfig
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *NodeValidateConfig) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, &t.OptsValidateConfig)
}

func (t *NodeValidateConfig) cmd() *cobra.Command {
	return &cobra.Command{
		Use:     "config",
		Short:   "verify the node configuration syntax is valid",
		Aliases: []string{"confi", "conf", "con", "co", "c"},
		Run: func(_ *cobra.Command, _ []string) {
			t.run()
		},
	}
}

func (t *NodeValidateConfig) run() {
	nodeaction.New(
		nodeaction.LocalFirst(),
		nodeaction.WithLocal(t.Global.Local),
		nodeaction.WithRemoteNodes(t.Global.NodeSelector),
		nodeaction.WithFormat(t.Global.Format),
		nodeaction.WithColor(t.Global.Color),
		nodeaction.WithServer(t.Global.Server),
		nodeaction.WithRemoteAction("push_asset"),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			return object.NewNode().ValidateConfig(t.OptsValidateConfig)
		}),
	).Do()
}
