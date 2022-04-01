package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/nodeaction"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
)

type (
	// CmdNodeComplianceAuto is the cobra flag set of the sysreport command.
	CmdNodeComplianceAuto struct {
		object.OptsNodeComplianceAuto
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdNodeComplianceAuto) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, &t.OptsNodeComplianceAuto)
}

func (t *CmdNodeComplianceAuto) cmd() *cobra.Command {
	return &cobra.Command{
		Use:   "auto",
		Short: "run compliance fixes on autofix modules, checks on other modules",
		Run: func(_ *cobra.Command, _ []string) {
			t.run()
		},
	}
}

func (t *CmdNodeComplianceAuto) run() {
	nodeaction.New(
		nodeaction.WithLocal(t.Global.Local),
		nodeaction.WithRemoteNodes(t.Global.NodeSelector),
		nodeaction.WithFormat(t.Global.Format),
		nodeaction.WithColor(t.Global.Color),
		nodeaction.WithServer(t.Global.Server),
		nodeaction.WithRemoteAction("compliance auto"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"format":    t.Global.Format,
			"force":     t.Force,
			"module":    t.Module,
			"moduleset": t.Moduleset,
			"attach":    t.Attach,
		}),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			return object.NewNode().ComplianceAuto(t.OptsNodeComplianceAuto)
		}),
	).Do()
}
