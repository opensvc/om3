package oxcmd

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
)

type (
	CmdObjectComplianceListModuleset struct {
		OptsGlobal
		Moduleset string
	}
)

// Run lists the modulesets the collector makes available to the selected
// objects.
func (t *CmdObjectComplianceListModuleset) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	return commoncmd.ObjectsAPIDo(mergedSelector, t.OptsGlobal.OptsGlobal, commoncmd.ObjectComplianceModulesets(t.Moduleset))
}
