package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/objectaction"
	"opensvc.com/opensvc/core/path"
)

type (
	// CmdObjectComplianceShowRuleset is the cobra flag set of the sysreport command.
	CmdObjectComplianceShowRuleset struct {
		object.OptsObjectComplianceShowRuleset
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectComplianceShowRuleset) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *CmdObjectComplianceShowRuleset) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:     "ruleset",
		Short:   "Show compliance rules applying to this node.",
		Aliases: []string{"rulese", "rules", "rule", "rul", "ru"},
		Run: func(_ *cobra.Command, _ []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdObjectComplianceShowRuleset) run(selector *string, kind string) {
	mergedSelector := mergeSelector(*selector, t.Global.ObjectSelector, kind, "")
	objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Global.Local),
		objectaction.WithColor(t.Global.Color),
		objectaction.WithFormat(t.Global.Format),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.Global.NodeSelector),
		objectaction.WithServer(t.Global.Server),
		objectaction.WithRemoteAction("compliance show ruleset"),
		objectaction.WithRemoteOptions(map[string]interface{}{
			"format":  t.Global.Format,
			"ruleset": t.Ruleset,
		}),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			if o, err := object.NewSvc(p); err != nil {
				return nil, err
			} else {
				return o.ComplianceShowRuleset(t.OptsObjectComplianceShowRuleset)
			}
		}),
	).Do()
}
