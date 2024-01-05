package oxcmd

import (
	"github.com/opensvc/om3/core/objectaction"
)

type (
	CmdKeystoreChange struct {
		OptsGlobal
		Key   string
		From  string
		Value string
	}
)

func (t *CmdKeystoreChange) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.WithColor(t.Color),
		objectaction.WithOutput(t.Output),
		objectaction.WithObjectSelector(mergedSelector),
	).Do()
}
