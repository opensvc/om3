package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/nodeaction"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
)

type (
	// NodePushAsset is the cobra flag set of the start command.
	NodePushAsset struct {
		object.OptsNodePushAsset
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *NodePushAsset) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, &t.OptsNodePushAsset)
}

func (t *NodePushAsset) InitAlt(parent *cobra.Command) {
	cmd := t.cmdAlt()
	parent.AddCommand(cmd)
	flag.Install(cmd, &t.OptsNodePushAsset)
}

func (t *NodePushAsset) cmd() *cobra.Command {
	return &cobra.Command{
		Use:     "asset",
		Short:   "Run the node discovery, push and print the result",
		Aliases: []string{"asse", "ass", "as"},
		Run: func(_ *cobra.Command, _ []string) {
			t.run()
		},
	}
}

func (t *NodePushAsset) cmdAlt() *cobra.Command {
	return &cobra.Command{
		Use:     "pushasset",
		Hidden:  true,
		Short:   "Run the node discovery, push and print the result",
		Aliases: []string{"pushasse", "pushass", "pushas", "pusha"},
		Run: func(_ *cobra.Command, _ []string) {
			t.run()
		},
	}
}

func (t *NodePushAsset) run() {
	nodeaction.New(
		nodeaction.WithLocal(t.Global.Local),
		nodeaction.WithRemoteNodes(t.Global.NodeSelector),
		nodeaction.WithFormat(t.Global.Format),
		nodeaction.WithColor(t.Global.Color),
		nodeaction.WithServer(t.Global.Server),
		nodeaction.WithRemoteAction("push_asset"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"format": t.Global.Format,
		}),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			return object.NewNode().PushAsset(), nil
		}),
	).Do()
}
