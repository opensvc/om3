package omcmd

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/object"
)

type CmdNodeDequeue struct {
	commoncmd.OptsNodeGlobal
}

func (t *CmdNodeDequeue) Run() error {
	if t.NodeSelector == "" {
		n, err := object.NewNode()
		if err != nil {
			return err
		}
		return n.Dequeue()
	}
	return t.asCommonCmd().Remote()
}

func (t *CmdNodeDequeue) asCommonCmd() *commoncmd.CmdNodeDequeue {
	return &commoncmd.CmdNodeDequeue{
		OptsNodeGlobal: commoncmd.OptsNodeGlobal{
			Color:          t.Color,
			Output:         t.Output,
			NodeSelector:   t.NodeSelector,
			IgnoreNotFound: t.IgnoreNotFound,
		},
	}
}
