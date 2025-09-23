package omcmd

import (
	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/util/hostname"
)

type (
	CmdNodeEvents struct {
		commoncmd.CmdNodeEvents
	}
)

func (t *CmdNodeEvents) Run() error {
	if t.Wait && t.Limit == 0 {
		t.Limit = 1
	}
	if t.NodeSelector == "" {
		t.NodeSelector = hostname.Hostname()
	}
	return t.DoNodes()
}
