package omcmd

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/nodeaction"
	"github.com/opensvc/om3/v3/util/xstrings"
)

type (
	CmdNodeComplianceShowModuleset struct {
		OptsGlobal
		Moduleset    string
		NodeSelector string
	}
)

func (t *CmdNodeComplianceShowModuleset) Run() error {
	return nodeaction.New(
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithDefaultOutput(""),
		nodeaction.WithFormat(t.Output),
		nodeaction.WithSort(t.Sort),
		nodeaction.WithColor(t.Color),
		nodeaction.WithLocalFunc(func() (any, error) {
			comp, err := newNodeCompliance()
			if err != nil {
				return nil, err
			}
			data, err := comp.GetData(xstrings.Split(t.Moduleset, ","))
			if err != nil {
				return nil, err
			}
			return data.ModulesetsTree(), nil
		}),
		nodeaction.WithRemoteFunc(commoncmd.NodeComplianceModulesetTreeRemote(t.Moduleset)),
	).Do()
}
