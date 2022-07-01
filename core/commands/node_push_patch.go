package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/nodeaction"
	"opensvc.com/opensvc/core/object"
)

type (
	// NodePushPatch is the cobra flag set of the start command.
	NodePushPatch struct {
		OptsGlobal
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *NodePushPatch) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *NodePushPatch) InitAlt(parent *cobra.Command) {
	cmd := t.cmdAlt()
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *NodePushPatch) cmd() *cobra.Command {
	return &cobra.Command{
		Use:     "patch",
		Short:   "Run the node installed patches discovery, push and print the result",
		Aliases: []string{"patc", "pat", "pa"},
		Run: func(_ *cobra.Command, _ []string) {
			t.run()
		},
	}
}

func (t *NodePushPatch) cmdAlt() *cobra.Command {
	return &cobra.Command{
		Use:     "pushpatch",
		Hidden:  true,
		Short:   "Run the node installed patches discovery, push and print the result",
		Aliases: []string{"pushpatc", "pushpat", "pushpa"},
		Run: func(_ *cobra.Command, _ []string) {
			t.run()
		},
	}
}

func (t *NodePushPatch) run() {
	nodeaction.New(
		nodeaction.WithLocal(t.Local),
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Format),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),
		nodeaction.WithRemoteAction("push_patch"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"format": t.Format,
		}),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			return object.NewNode().PushPatch()
		}),
	).Do()
}
