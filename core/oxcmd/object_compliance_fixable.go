package oxcmd

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/util/compliance"
)

type (
	CmdObjectComplianceFixable struct {
		OptsGlobal
		Moduleset    string
		Module       string
		NodeSelector string
		Force        bool
		Attach       bool
	}
)

func (t *CmdObjectComplianceFixable) Run(kind string) error {
	opts := commoncmd.ComplianceRunOptions{
		Modulesets: t.Moduleset,
		Modules:    t.Module,
		Force:      t.Force,
		Attach:     t.Attach,
	}
	return doObjectComplianceRun(kind, compliance.ActionFixable, opts, t.NodeSelector, t.OptsGlobal)
}
