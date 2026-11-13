package omcmd

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/util/compliance"
)

type (
	CmdNodeComplianceAuto struct {
		OptsGlobal
		Moduleset    string
		Module       string
		NodeSelector string
		Force        bool
		Attach       bool
	}
)

func (t *CmdNodeComplianceAuto) Run() error {
	opts := commoncmd.ComplianceRunOptions{
		Modulesets: t.Moduleset,
		Modules:    t.Module,
		Force:      t.Force,
		Attach:     t.Attach,
	}
	return doNodeComplianceRun(compliance.ActionAuto, opts, t.NodeSelector, t.OptsGlobal)
}
