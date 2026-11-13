package omcmd

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/util/compliance"
)

type (
	CmdNodeComplianceFixable struct {
		OptsGlobal
		Moduleset    string
		Module       string
		NodeSelector string
		Force        bool
		Attach       bool
	}
)

func (t *CmdNodeComplianceFixable) Run() error {
	opts := commoncmd.ComplianceRunOptions{
		Modulesets: t.Moduleset,
		Modules:    t.Module,
		Force:      t.Force,
		Attach:     t.Attach,
	}
	return doNodeComplianceRun(compliance.ActionFixable, opts, t.NodeSelector, t.OptsGlobal)
}
