package commands

import (
	"context"

	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/actioncontext"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/nodeaction"
	"opensvc.com/opensvc/core/object"
)

type (
	// NodeGet is the cobra flag set of the start command.
	NodeGet struct {
		OptsGlobal
		OptsLock
		Keyword string `flag:"kw"`
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *NodeGet) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *NodeGet) cmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get",
		Short: "get a configuration key value",
		Run: func(_ *cobra.Command, _ []string) {
			t.run()
		},
	}
}

func (t *NodeGet) run() {
	nodeaction.New(
		nodeaction.LocalFirst(),
		nodeaction.WithLocal(t.Local),
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Format),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),
		nodeaction.WithRemoteAction("get"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"kw": t.Keyword,
		}),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			n := object.NewNode()
			ctx := context.Background()
			ctx = actioncontext.WithLockDisabled(ctx, t.Disable)
			ctx = actioncontext.WithLockTimeout(ctx, t.Timeout)
			return n.Get(ctx, t.Keyword)

		}),
	).Do()
}
