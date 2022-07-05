package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/nodeaction"
	"opensvc.com/opensvc/core/object"
)

type (
	// CmdNodeComplianceAuto is the cobra flag set of the sysreport command.
	CmdNodeComplianceAuto struct {
		OptsGlobal
		OptModuleset
		OptModule
		OptForce
		OptAttach
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdNodeComplianceAuto) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
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
		nodeaction.WithLocal(t.Local),
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Format),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),
		nodeaction.WithRemoteAction("compliance auto"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"format":    t.Format,
			"force":     t.Force,
			"module":    t.Module,
			"moduleset": t.Moduleset,
			"attach":    t.Attach,
		}),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			comp, err := object.NewNode().NewCompliance()
			if err != nil {
				return nil, err
			}
			run := comp.NewRun()
			run.SetModulesetsExpr(t.Moduleset)
			run.SetModulesExpr(t.Module)
			run.SetForce(t.Force)
			run.SetAttach(t.Attach)
			err = run.Auto()
			return run, err
		}),
	).Do()
}
