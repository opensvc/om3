package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/objectaction"
	"opensvc.com/opensvc/core/path"
)

type (
	// CmdObjectComplianceFixable is the cobra flag set of the sysreport command.
	CmdObjectComplianceFixable struct {
		OptsGlobal
		OptModuleset
		OptModule
		OptForce
		OptAttach
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectComplianceFixable) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *CmdObjectComplianceFixable) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:     "fixable",
		Short:   "run compliance fixable-tests",
		Aliases: []string{"fixabl", "fixab", "fixa"},
		Run: func(_ *cobra.Command, _ []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdObjectComplianceFixable) run(selector *string, kind string) {
	mergedSelector := mergeSelector(*selector, t.ObjectSelector, kind, "")
	objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Local),
		objectaction.WithColor(t.Color),
		objectaction.WithFormat(t.Format),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithServer(t.Server),
		objectaction.WithRemoteAction("compliance fixable"),
		objectaction.WithRemoteOptions(map[string]interface{}{
			"format":    t.Format,
			"force":     t.Force,
			"module":    t.Module,
			"moduleset": t.Moduleset,
			"attach":    t.Attach,
		}),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			if o, err := object.NewSvc(p); err != nil {
				return nil, err
			} else {
				comp, err := o.NewCompliance()
				if err != nil {
					return nil, err
				}
				run := comp.NewRun()
				run.SetModulesetsExpr(t.Moduleset)
				run.SetModulesExpr(t.Module)
				run.SetForce(t.Force)
				run.SetAttach(t.Attach)
				err = run.Fixable()
				return run, err
			}
		}),
	).Do()
}
