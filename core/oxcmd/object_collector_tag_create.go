package oxcmd

import (
	"github.com/opensvc/om3/core/objectaction"
)

type (
	CmdObjectCollectorTagCreate struct {
		OptsGlobal
		Name    string
		Data    *string
		Exclude *string
	}
)

func (t *CmdObjectCollectorTagCreate) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithOutput(t.Output),
		objectaction.WithColor(t.Color),
	).Do()
}
