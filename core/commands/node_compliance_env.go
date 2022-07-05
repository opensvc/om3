package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/nodeaction"
	"opensvc.com/opensvc/core/object"
)

type (
	// CmdNodeComplianceEnv is the cobra flag set of the sysreport command.
	CmdNodeComplianceEnv struct {
		OptsGlobal
		OptModuleset
		OptModule
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdNodeComplianceEnv) Init(parent *cobra.Command) {
	cmd := t.cmd()
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *CmdNodeComplianceEnv) cmd() *cobra.Command {
	return &cobra.Command{
		Use:     "env",
		Short:   "Show the environment variables set during a compliance module run.",
		Aliases: []string{"en"},
		Run: func(_ *cobra.Command, _ []string) {
			t.run()
		},
	}
}

func (t *CmdNodeComplianceEnv) run() {
	nodeaction.New(
		nodeaction.WithLocal(t.Local),
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Format),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),
		nodeaction.WithRemoteAction("compliance env"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"format":    t.Format,
			"moduleset": t.Moduleset,
			"module":    t.Module,
		}),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			comp, err := object.NewNode().NewCompliance()
			if err != nil {
				return nil, err
			}
			run := comp.NewRun()
			run.SetModulesetsExpr(t.Moduleset)
			run.SetModulesExpr(t.Module)
			return run.Env()
		}),
	).Do()
}
