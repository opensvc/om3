package oxcmd

import (
	"fmt"

	"github.com/opensvc/om3/core/clientcontext"
	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/util/hostname"
)

type (
	CmdNodeEvents struct {
		commoncmd.CmdNodeEvents
	}
)

func (t *CmdNodeEvents) Run() error {
	if !clientcontext.IsSet() && t.NodeSelector == "" {
		t.NodeSelector = hostname.Hostname()
	}
	if t.NodeSelector == "" {
		return fmt.Errorf("--node must be specified")
	}
	return t.DoNodes()
}
