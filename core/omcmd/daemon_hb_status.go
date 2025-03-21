package omcmd

import (
	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/util/hostname"
)

type (
	CmdDaemonHeartbeatStatus struct {
		OptsGlobal
		NodeSelector string
		PeerSelector string
		Name         string
	}
)

func (t *CmdDaemonHeartbeatStatus) Run() error {
	cmd := commoncmd.CmdDaemonHeartbeatStatus{
		OptsGlobal: commoncmd.OptsGlobal{
			Color:  t.Color,
			Output: t.Output,
		},
		NodeSelector: t.NodeSelector,
		PeerSelector: t.PeerSelector,
		Name:         t.Name,
	}
	if t.NodeSelector == "" {
		cmd.NodeSelector = hostname.Hostname()
	}
	return cmd.Run()
}
