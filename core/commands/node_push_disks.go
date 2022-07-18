package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/nodeaction"
	"opensvc.com/opensvc/core/object"
)

type (
	// NodePushDisks is the cobra flag set of the start command.
	NodePushDisks struct {
		OptsGlobal
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *NodePushDisks) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *NodePushDisks) InitAlt(parent *cobra.Command) {
	cmd := t.cmdAlt()
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *NodePushDisks) cmd() *cobra.Command {
	return &cobra.Command{
		Use:     "disks",
		Short:   "Run the disk discovery, push and print the result",
		Aliases: []string{"disk", "dis", "di"},
		Run: func(_ *cobra.Command, _ []string) {
			t.run()
		},
	}
}

func (t *NodePushDisks) cmdAlt() *cobra.Command {
	return &cobra.Command{
		Use:     "pushdisks",
		Hidden:  true,
		Short:   "Run the disk discovery, push and print the result",
		Aliases: []string{"pushdisk", "pushdis", "pushdi"},
		Run: func(_ *cobra.Command, _ []string) {
			t.run()
		},
	}
}

func (t *NodePushDisks) run() {
	nodeaction.New(
		nodeaction.WithLocal(t.Local),
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Format),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),
		nodeaction.WithRemoteAction("push_disks"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"format": t.Format,
		}),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			n, err := object.NewNode()
			if err != nil {
				return nil, err
			}
			return n.PushDisks()
		}),
	).Do()
}
