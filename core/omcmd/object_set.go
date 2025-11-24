package omcmd

import (
	"github.com/opensvc/om3/core/commoncmd"
)

type (
	CmdObjectSet struct {
		OptsGlobal
		commoncmd.OptsLock
		Local      bool
		KeywordOps []string
	}
)

func (t *CmdObjectSet) Run(kind string) error {
	cmd := &CmdObjectConfigUpdate{
		OptsGlobal: t.OptsGlobal,
		OptsLock:   t.OptsLock,
		Local:      t.Local,
		Set:        t.KeywordOps,
	}
	return cmd.Run(kind)
}
