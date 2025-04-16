package oxcmd

import (
	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/core/objectaction"
)

type (
	CmdObjectComplianceAuto struct {
		OptsGlobal
		Moduleset    string
		Module       string
		NodeSelector string
		Force        bool
		Attach       bool
	}
)

func (t *CmdObjectComplianceAuto) Run(selector, kind string) error {
	mergedSelector := commoncmd.MergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.WithColor(t.Color),
		objectaction.WithOutput(t.Output),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.NodeSelector),
	).Do()
}
