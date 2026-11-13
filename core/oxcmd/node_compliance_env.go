package oxcmd

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/nodeaction"
)

type (
	CmdNodeComplianceEnv struct {
		OptsGlobal
		Moduleset    string
		Module       string
		NodeSelector string
	}
)

func (t *CmdNodeComplianceEnv) Run() error {
	return nodeaction.New(
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithDefaultOutput(""),
		nodeaction.WithFormat(t.Output),
		nodeaction.WithSort(t.Sort),
		nodeaction.WithColor(t.Color),
		nodeaction.WithRemoteFunc(commoncmd.NodeComplianceEnvRemote(t.Moduleset, t.Module)),
	).Do()
}
