package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/objectaction"
	"opensvc.com/opensvc/core/path"
)

type (
	// CmdObjectComplianceShowModuleset is the cobra flag set of the sysreport command.
	CmdObjectComplianceShowModuleset struct {
		OptsGlobal
		object.OptsObjectComplianceShowModuleset
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectComplianceShowModuleset) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *CmdObjectComplianceShowModuleset) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:     "moduleset",
		Short:   "Show compliance moduleset and modules attached to this node.",
		Aliases: []string{"modulese", "modules", "module", "modul", "modu", "mod", "mo"},
		Run: func(_ *cobra.Command, _ []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdObjectComplianceShowModuleset) run(selector *string, kind string) {
	mergedSelector := mergeSelector(*selector, t.ObjectSelector, kind, "")
	objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Local),
		objectaction.WithColor(t.Color),
		objectaction.WithFormat(t.Format),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithServer(t.Server),
		objectaction.WithRemoteAction("compliance show moduleset"),
		objectaction.WithRemoteOptions(map[string]interface{}{
			"format":    t.Format,
			"moduleset": t.Moduleset,
		}),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			if o, err := object.NewSvc(p); err != nil {
				return nil, err
			} else {
				return o.ComplianceShowModuleset(t.OptsObjectComplianceShowModuleset)
			}
		}),
	).Do()
}
