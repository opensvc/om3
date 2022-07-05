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
	// NodeEval is the cobra flag set of the start command.
	NodeEval struct {
		OptsGlobal
		OptsLock
		Keyword     string `flag:"kw"`
		Impersonate string `flag:"impersonate"`
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *NodeEval) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *NodeEval) cmd() *cobra.Command {
	return &cobra.Command{
		Use:   "eval",
		Short: "evaluate a configuration key value",
		Run: func(_ *cobra.Command, _ []string) {
			t.run()
		},
	}
}

func (t *NodeEval) run() {
	nodeaction.New(
		nodeaction.LocalFirst(),
		nodeaction.WithLocal(t.Local),
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Format),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),
		nodeaction.WithRemoteAction("push_asset"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"kw":          t.Keyword,
			"impersonate": t.Impersonate,
		}),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			n := object.NewNode()
			ctx := context.Background()
			ctx = actioncontext.WithLockDisabled(ctx, t.Disable)
			ctx = actioncontext.WithLockTimeout(ctx, t.Timeout)
			return n.EvalAs(ctx, t.Keyword, t.Impersonate)
		}),
	).Do()
}
