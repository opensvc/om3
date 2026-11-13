package oxcmd

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
)

type (
	CmdObjectComplianceAttachModuleset struct {
		OptsGlobal
		Moduleset string
	}
)

// Run attaches the moduleset to the selected objects, on the collector.
func (t *CmdObjectComplianceAttachModuleset) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	return commoncmd.ObjectsAPIDo(mergedSelector, t.OptsGlobal.OptsGlobal, commoncmd.ObjectComplianceModulesetAttachment(t.Moduleset, true))
}
