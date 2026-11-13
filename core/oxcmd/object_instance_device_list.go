package oxcmd

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
)

type (
	CmdObjectInstanceDeviceList struct {
		OptsGlobal
		NodeSelector string
		Roles        string
	}
)

// Run lists the devices of the instances of the selected objects, on the
// nodes selected or on all of them, each asked of its node.
func (t *CmdObjectInstanceDeviceList) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	selector := t.NodeSelector
	if selector == "" {
		selector = "*"
	}
	return commoncmd.InstanceDeviceList(mergedSelector, selector, t.Roles, t.OptsGlobal.OptsGlobal)
}
