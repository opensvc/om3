package omcmd

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/util/hostname"
)

type (
	CmdDaemonEvents struct {
		commoncmd.CmdDaemonEvents
	}
)

func (t *CmdDaemonEvents) Run() error {
	if t.NodeSelector == "" {
		t.NodeSelector = hostname.Hostname()
	}
	return t.DoNodes()
}
