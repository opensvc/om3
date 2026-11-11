package resapp

import (
	"fmt"

	"github.com/opensvc/om3/v3/util/capexec"
)

// toCaps returns the cappings and limits om exec applies to the action
// command, including the identity it demotes to. The credential is
// decided by the policy in credential(), not by the user and group
// keywords alone.
func (t *T) toCaps(cred execCredential) capexec.T {
	xo := capexec.T{}
	//xo.LoadPG(*t.GetPG())
	pg := *t.GetPG()
	if pg.ID != "" {
		xo.PGID = &pg.ID
	}
	if cred.User != "" {
		xo.User = &cred.User
	}
	if cred.Group != "" {
		xo.Group = &cred.Group
	}
	if t.Umask != nil {
		umask := fmt.Sprintf("%03o", uint32(*t.Umask))
		xo.Umask = &umask
	}
	xo.LoadLimit(t.Limit)
	return xo
}
