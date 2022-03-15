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
		object.OptsNodeRegister
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdNodeRegister) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, &t.OptsNodeRegister)
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
		nodeaction.WithLocal(t.Global.Local),
		nodeaction.WithRemoteNodes(t.Global.NodeSelector),
		nodeaction.WithFormat(t.Global.Format),
		nodeaction.WithColor(t.Global.Color),
		nodeaction.WithServer(t.Global.Server),
		nodeaction.WithRemoteAction("register"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"format": t.Global.Format,
		}),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			return nil, object.NewNode().Register(t.OptsNodeRegister)
		}),
	).Do()
}
