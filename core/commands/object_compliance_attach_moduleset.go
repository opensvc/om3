package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/objectaction"
	"opensvc.com/opensvc/core/path"
)

type (
	// CmdObjectComplianceAttachModuleset is the cobra flag set of the sysreport command.
	CmdObjectComplianceAttachModuleset struct {
		object.OptsObjectComplianceAttachModuleset
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectComplianceAttachModuleset) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *CmdObjectComplianceAttachModuleset) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:     "moduleset",
		Short:   "attach compliance moduleset to this node.",
		Long:    "modules of attached modulesets are checked on schedule.",
		Aliases: []string{"modulese", "modules", "module", "modul", "modu", "mod", "mo"},
		Run: func(_ *cobra.Command, _ []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdObjectComplianceAttachModuleset) run(selector *string, kind string) {
	mergedSelector := mergeSelector(*selector, t.Global.ObjectSelector, kind, "")
	objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Global.Local),
		objectaction.WithColor(t.Global.Color),
		objectaction.WithFormat(t.Global.Format),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.Global.NodeSelector),
		objectaction.WithServer(t.Global.Server),
		objectaction.WithRemoteAction("compliance attach moduleset"),
		objectaction.WithRemoteOptions(map[string]interface{}{
			"format": t.Global.Format,
		}),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			if o, err := object.NewSvc(p); err != nil {
				return nil, err
			} else {
				return nil, o.ComplianceAttachModuleset(t.OptsObjectComplianceAttachModuleset)
			}
		}),
	).Do()
}
