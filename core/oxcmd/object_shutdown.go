package oxcmd

import (
	"github.com/opensvc/om3/core/objectaction"
)

type (
	CmdObjectShutdown struct {
		OptsGlobal
		OptsLock
		OptsResourceSelector
		OptTo
		Force        bool
		NodeSelector string
	}
)

func (t *CmdObjectShutdown) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRID(t.RID),
		objectaction.WithTag(t.Tag),
		objectaction.WithSubset(t.Subset),
		objectaction.WithOutput(t.Output),
		objectaction.WithColor(t.Color),
		objectaction.WithRemoteNodes(t.NodeSelector),
	).Do()
}
