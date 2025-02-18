package oxcmd

import (
	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/util/render"
)

type (
	CmdNodeLogs struct {
		OptsGlobal
		OptsLogs
		NodeSelector string
	}
)

func (t *CmdNodeLogs) Run() error {
	render.SetColor(t.Color)
	if t.NodeSelector == "" {
		t.NodeSelector = "*"
	}
	return t.asCommonCmd().Remote()
}

func (t *CmdNodeLogs) asCommonCmd() *commoncmd.CmdNodeLogs {
	return &commoncmd.CmdNodeLogs{
		OptsGlobal: commoncmd.OptsGlobal{
			Color:          t.Color,
			Output:         t.Output,
			ObjectSelector: t.ObjectSelector,
		},
		OptsLogs: commoncmd.OptsLogs{
			Follow: t.Follow,
			Lines:  t.Lines,
			Filter: t.Filter,
		},
		NodeSelector: t.NodeSelector,
	}
}
