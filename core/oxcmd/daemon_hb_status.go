package oxcmd

import "github.com/opensvc/om3/core/commoncmd"

type (
	CmdDaemonHeartbeatStatus struct {
		OptsGlobal
		NodeSelector string
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
		Name:         t.Name,
	}
	if t.NodeSelector == "" {
		cmd.NodeSelector = "*"
	}
	return cmd.Run()
}
