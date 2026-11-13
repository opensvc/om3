package omcmd

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/util/compliance"
)

type (
	CmdNodeComplianceCheck struct {
		OptsGlobal
		Moduleset    string
		Module       string
		NodeSelector string
		Force        bool
		Attach       bool
	}
)

func (t *CmdNodeComplianceCheck) Run() error {
	opts := commoncmd.ComplianceRunOptions{
		Modulesets: t.Moduleset,
		Modules:    t.Module,
		Force:      t.Force,
		Attach:     t.Attach,
	}
	return doNodeComplianceRun(compliance.ActionCheck, opts, t.NodeSelector, t.OptsGlobal)
}
