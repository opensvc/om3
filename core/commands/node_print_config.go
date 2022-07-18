package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/nodeaction"
	"opensvc.com/opensvc/core/object"
)

type (
	// NodePrintConfig is the cobra flag set of the start command.
	NodePrintConfig struct {
		OptsGlobal
		Eval        bool   `flag:"eval"`
		Impersonate string `flag:"impersonate"`
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *NodePrintConfig) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *NodePrintConfig) cmd() *cobra.Command {
	return &cobra.Command{
		Use:     "config",
		Short:   "get a configuration key value",
		Aliases: []string{"confi", "conf", "con", "co", "c", "cf", "cfg"},
		Run: func(_ *cobra.Command, _ []string) {
			t.run()
		},
	}
}

func (t *NodePrintConfig) run() {
	nodeaction.New(
		nodeaction.LocalFirst(),
		nodeaction.WithLocal(t.Local),
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Format),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),
		nodeaction.WithRemoteAction("print config"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"impersonate": t.Impersonate,
			"eval":        t.Eval,
		}),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			n, err := object.NewNode()
			if err != nil {
				return nil, err
			}
			switch {
			case t.Eval:
				return n.EvalConfigAs(t.Impersonate)
			default:
				return n.PrintConfig()
			}
		}),
	).Do()
}
