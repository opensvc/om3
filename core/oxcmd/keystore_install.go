package oxcmd

import (
	"github.com/opensvc/om3/core/objectaction"
)

type (
	CmdKeystoreInstall struct {
		OptsGlobal
		NodeSelector string
		Key          string
	}
)

func (t *CmdKeystoreInstall) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.WithColor(t.Color),
		objectaction.WithOutput(t.Output),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.NodeSelector),
	).Do()
}
