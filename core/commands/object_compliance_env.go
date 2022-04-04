package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/objectaction"
	"opensvc.com/opensvc/core/path"
)

type (
	// CmdObjectComplianceEnv is the cobra flag set of the sysreport command.
	CmdObjectComplianceEnv struct {
		object.OptsObjectComplianceEnv
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectComplianceEnv) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *CmdObjectComplianceEnv) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:     "env",
		Short:   "Show the environment variables set during a compliance module run.",
		Aliases: []string{"en"},
		Run: func(_ *cobra.Command, _ []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdObjectComplianceEnv) run(selector *string, kind string) {
	mergedSelector := mergeSelector(*selector, t.Global.ObjectSelector, kind, "")
	objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Global.Local),
		objectaction.WithColor(t.Global.Color),
		objectaction.WithFormat(t.Global.Format),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.Global.NodeSelector),
		objectaction.WithServer(t.Global.Server),
		objectaction.WithRemoteAction("compliance env"),
		objectaction.WithRemoteOptions(map[string]interface{}{
			"format":    t.Global.Format,
			"moduleset": t.Moduleset,
			"module":    t.Module,
		}),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			if o, err := object.NewSvc(p); err != nil {
				return nil, err
			} else {
				return o.ComplianceEnv(t.OptsObjectComplianceEnv)
			}
		}),
	).Do()
}
