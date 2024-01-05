package oxcmd

import (
	"github.com/opensvc/om3/core/objectaction"
)

type (
	CmdObjectCollectorTagShow struct {
		OptsGlobal
		Verbose bool
	}
)

func (t *CmdObjectCollectorTagShow) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithOutput(t.Output),
		objectaction.WithColor(t.Color),
	).Do()
}
