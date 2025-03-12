package oxcmd

import (
	"github.com/opensvc/om3/core/objectaction"
)

type (
	CmdObjectCertificatePKCS struct {
		OptsGlobal
	}
)

func (t *CmdObjectCertificatePKCS) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithColor(t.Color),
		objectaction.WithOutput(t.Output),
		objectaction.WithObjectSelector(mergedSelector),
	).Do()
}
