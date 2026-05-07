package oxcmd

import (
	"fmt"

	"github.com/opensvc/om3/v3/core/clientcontext"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/util/hostname"
)

type (
	CmdDaemonEvents struct {
		commoncmd.CmdDaemonEvents
	}
)

func (t *CmdDaemonEvents) Run() error {
	if !clientcontext.IsSet() && t.NodeSelector == "" {
		t.NodeSelector = hostname.Hostname()
	}
	if t.NodeSelector == "" {
		return fmt.Errorf("--node must be specified")
	}
	return t.DoNodes()
}
