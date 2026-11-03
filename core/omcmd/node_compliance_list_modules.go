package omcmd

import (
	"github.com/opensvc/om3/v3/core/nodeaction"
	"github.com/opensvc/om3/v3/util/compliance"
)

type (
	CmdNodeComplianceListModules struct {
		OptsGlobal
		NodeSelector string
	}
)

func (t *CmdNodeComplianceListModules) Run() error {
	return nodeaction.New(
		nodeaction.WithFormat(t.Output),
		nodeaction.WithSort(t.Sort),
		nodeaction.WithColor(t.Color),
		nodeaction.WithLocalFunc(func() (interface{}, error) {
			// The modules installed on the node: no collector is
			// asked, none is needed.
			return compliance.New().ListModuleNames()
		}),
	).Do()
}
