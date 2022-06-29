package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/nodeaction"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
)

type (
	// NodePushPkg is the cobra flag set of the start command.
	NodePushPkg struct {
		OptsGlobal
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *NodePushPkg) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *NodePushPkg) InitAlt(parent *cobra.Command) {
	cmd := t.cmdAlt()
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *NodePushPkg) cmd() *cobra.Command {
	return &cobra.Command{
		Use:     "pkg",
		Short:   "Run the node installed packages discovery, push and print the result",
		Aliases: []string{"pk"},
		Run: func(_ *cobra.Command, _ []string) {
			t.run()
		},
	}
}

func (t *NodePushPkg) cmdAlt() *cobra.Command {
	return &cobra.Command{
		Use:     "pushpkg",
		Hidden:  true,
		Short:   "Run the node installed packages discovery, push and print the result",
		Aliases: []string{"pushpk"},
		Run: func(_ *cobra.Command, _ []string) {
			t.run()
		},
	}
}

func (t *NodePushPkg) run() {
	nodeaction.New(
		nodeaction.WithLocal(t.Local),
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Format),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),
		nodeaction.WithRemoteAction("push_pkg"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"format": t.Format,
		}),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			return object.NewNode().PushPkg()
		}),
	).Do()
}
