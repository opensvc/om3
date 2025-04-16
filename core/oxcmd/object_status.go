package oxcmd

import (
	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/core/objectaction"
)

type (
	CmdObjectStatus struct {
		OptsGlobal
		commoncmd.OptsLock
		Refresh      bool
		NodeSelector string
	}
)

func (t *CmdObjectStatus) Run(selector, kind string) error {
	mergedSelector := commoncmd.MergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithOutput(t.Output),
		objectaction.WithColor(t.Color),
		objectaction.WithRemoteNodes(t.NodeSelector),
	).Do()
}
