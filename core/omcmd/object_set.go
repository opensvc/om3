package omcmd

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
)

type (
	CmdObjectSet struct {
		OptsGlobal
		commoncmd.OptsLock
		Local      bool
		KeywordOps []string
		Force      bool
	}
)

func (t *CmdObjectSet) Run(kind string) error {
	cmd := &CmdObjectConfigUpdate{
		OptsGlobal: t.OptsGlobal,
		OptsLock:   t.OptsLock,
		Local:      t.Local,
		Force:      t.Force,
		Set:        t.KeywordOps,
	}
	return cmd.Run(kind)
}
