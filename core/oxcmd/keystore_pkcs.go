package oxcmd

import (
	"github.com/opensvc/om3/core/objectaction"
)

type (
	CmdPKCS struct {
		OptsGlobal
	}
)

func (t *CmdPKCS) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithColor(t.Color),
		objectaction.WithOutput(t.Output),
		objectaction.WithObjectSelector(mergedSelector),
	).Do()
}
