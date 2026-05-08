package oxcmd

import "github.com/opensvc/om3/v3/core/commoncmd"

type CmdNodeDequeue struct {
	commoncmd.OptsNodeGlobal
}

func (t *CmdNodeDequeue) Run() error {
	return t.asCommonCmd().Remote()
}

func (t *CmdNodeDequeue) asCommonCmd() *commoncmd.CmdNodeDequeue {
	return &commoncmd.CmdNodeDequeue{
		OptsNodeGlobal: commoncmd.OptsNodeGlobal{
			Color:        t.Color,
			Output:       t.Output,
			NodeSelector: t.NodeSelector,
		},
	}
}
