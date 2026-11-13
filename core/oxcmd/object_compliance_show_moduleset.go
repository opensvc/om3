package oxcmd

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
)

type (
	CmdObjectComplianceShowModuleset struct {
		OptsGlobal
		Moduleset string
	}
)

// Run shows the modulesets attached to the selected objects, with their
// modules.
func (t *CmdObjectComplianceShowModuleset) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	return commoncmd.ObjectsAPIDo(mergedSelector, t.OptsGlobal.OptsGlobal, commoncmd.ObjectComplianceModulesetTree(t.Moduleset))
}
