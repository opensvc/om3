package omcmd

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
)

type (
	CmdObjectUnset struct {
		OptsGlobal
		commoncmd.OptsLock
		Local    bool
		Keywords []string
		Sections []string
	}
)

func (t *CmdObjectUnset) Run(kind string) error {
	cmd := &CmdObjectConfigUpdate{
		OptsGlobal: t.OptsGlobal,
		OptsLock:   t.OptsLock,
		Local:      t.Local,
		Unset:      t.Keywords,
		Delete:     t.Sections,
	}
	return cmd.Run(kind)
}
