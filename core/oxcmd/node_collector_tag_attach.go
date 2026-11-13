package oxcmd

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
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
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithDefaultOutput(""),
		nodeaction.WithFormat(t.Output),
		nodeaction.WithSort(t.Sort),
		nodeaction.WithColor(t.Color),
		nodeaction.WithRemoteFunc(commoncmd.NodeCollectorTagAttachRemote(t.Name, t.AttachData)),
	).Do()
}
