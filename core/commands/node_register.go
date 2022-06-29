package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/nodeaction"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
)

type (
	// CmdNodeRegister is the cobra flag set of the start command.
	CmdNodeRegister struct {
		OptsGlobal
		object.OptsNodeRegister
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdNodeRegister) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *CmdNodeRegister) cmd() *cobra.Command {
	return &cobra.Command{
		Use:     "register",
		Short:   "Obtain a registration id from the collector. This is is then used to authenticate the node in collector communications.",
		Aliases: []string{"registe", "regist", "regis", "regi", "reg", "re"},
		Run: func(_ *cobra.Command, _ []string) {
			t.run()
		},
	}
}

func (t *CmdNodeRegister) run() {
	nodeaction.New(
		nodeaction.WithLocal(t.Local),
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Format),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),
		nodeaction.WithRemoteAction("register"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"format": t.Format,
		}),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			return nil, object.NewNode().Register(t.OptsNodeRegister)
		}),
	).Do()
}
