package oxcmd

import (
	"github.com/opensvc/om3/v3/core/nodeaction"
)

type (
	CmdNodeCollectorTagAttach struct {
		OptsGlobal
		Name         string
		AttachData   *string
		NodeSelector string
	}
)

func (t *CmdNodeCollectorTagAttach) Run() error {
	return nodeaction.New(
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
		nodeaction.WithRemoteNodes(t.NodeSelector),
	).Do()
}
