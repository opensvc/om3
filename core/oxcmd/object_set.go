package oxcmd

import (
	"github.com/opensvc/om3/core/commoncmd"
)

type (
	CmdObjectSet struct {
		OptsGlobal
		commoncmd.OptsLock
		KeywordOps []string
	}
)

func (t *CmdObjectSet) Run(kind string) error {
	cmd := &CmdObjectConfigUpdate{
		OptsGlobal: t.OptsGlobal,
		OptsLock:   t.OptsLock,
		Set:        t.KeywordOps,
	}
	return cmd.Run(kind)
}
