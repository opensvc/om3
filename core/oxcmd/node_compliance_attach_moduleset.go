package oxcmd

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/nodeaction"
)

type (
	CmdNodeComplianceAttachModuleset struct {
		OptsGlobal
		Moduleset    string
		NodeSelector string
	}
)

func (t *CmdNodeComplianceAttachModuleset) Run() error {
	return nodeaction.New(
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithDefaultOutput(""),
		nodeaction.WithFormat(t.Output),
		nodeaction.WithSort(t.Sort),
		nodeaction.WithColor(t.Color),
		nodeaction.WithRemoteFunc(commoncmd.NodeComplianceModulesetAttachmentRemote(t.Moduleset, true)),
	).Do()
}
