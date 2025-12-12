package oxcmd

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/objectaction"
)

type (
	CmdObjectComplianceFix struct {
		OptsGlobal
		Moduleset    string
		Module       string
		NodeSelector string
		Force        bool
		Attach       bool
	}
)

func (t *CmdObjectComplianceFix) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.WithColor(t.Color),
		objectaction.WithOutput(t.Output),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.NodeSelector),
	).Do()
}
