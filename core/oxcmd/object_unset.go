package oxcmd

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
)

type (
	CmdObjectUnset struct {
		OptsGlobal
		commoncmd.OptsLock
		Keywords []string
		Sections []string
		Force    bool
	}
)

func (t *CmdObjectUnset) Run(kind string) error {
	cmd := &CmdObjectConfigUpdate{
		OptsGlobal: t.OptsGlobal,
		OptsLock:   t.OptsLock,
		Unset:      t.Keywords,
		Delete:     t.Sections,
		Force:      t.Force,
	}
	return cmd.Run(kind)
}
