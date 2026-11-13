package oxcmd

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
)

type (
	CmdObjectComplianceDetachModuleset struct {
		OptsGlobal
		Moduleset string
	}
)

// Run detaches the moduleset from the selected objects, on the collector.
func (t *CmdObjectComplianceDetachModuleset) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	return commoncmd.ObjectsAPIDo(mergedSelector, t.OptsGlobal.OptsGlobal, commoncmd.ObjectComplianceModulesetAttachment(t.Moduleset, false))
}
