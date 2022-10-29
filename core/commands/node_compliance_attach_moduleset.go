package commands

import (
	"opensvc.com/opensvc/core/nodeaction"
	"opensvc.com/opensvc/core/object"
)

type (
	CmdNodeComplianceAttachModuleset struct {
		OptsGlobal
		Moduleset string
	}
)

func (t *CmdNodeComplianceAttachModuleset) Run() error {
	return nodeaction.New(
		nodeaction.WithLocal(t.Local),
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Format),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),
		nodeaction.WithRemoteAction("compliance attach moduleset"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"format": t.Format,
		}),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			n, err := object.NewNode()
			if err != nil {
				return nil, err
			}
			comp, err := n.NewCompliance()
			if err != nil {
				return nil, err
			}
			return nil, comp.AttachModuleset(t.Moduleset)
		}),
	).Do()
}
