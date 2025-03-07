package oxcmd

import (
	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/util/render"
)

type (
	CmdObjectLogs struct {
		OptsGlobal
		commoncmd.OptsLogs
		NodeSelector string
	}
)

func (t *CmdObjectLogs) Run(selector, kind string) error {
	render.SetColor(t.Color)
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "**")
	return t.asCommonCmd().Remote(mergedSelector)
}

func (t *CmdObjectLogs) asCommonCmd() *commoncmd.CmdObjectLogs {
	return &commoncmd.CmdObjectLogs{
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
