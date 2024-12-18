package oxcmd

import (
	"github.com/opensvc/om3/core/nodeaction"
)

type (
	CmdNodeRegister struct {
		OptsGlobal
		User         string
		Password     string
		App          string
		NodeSelector string
	}
)

func (t *CmdNodeRegister) Run() error {
	return nodeaction.New(
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
	).Do()
}
