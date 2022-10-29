package commands

import (
	"opensvc.com/opensvc/core/nodeaction"
	"opensvc.com/opensvc/core/object"
)

type (
	CmdNodeComplianceDetachModuleset struct {
		OptsGlobal
		Moduleset string
	}
)

func (t *CmdNodeComplianceDetachModuleset) Run() error {
	return nodeaction.New(
		nodeaction.WithLocal(t.Local),
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Format),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),
		nodeaction.WithRemoteAction("compliance detach moduleset"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"format":    t.Format,
			"moduleset": t.Moduleset,
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
			return nil, comp.DetachModuleset(t.Moduleset)
		}),
	).Do()
}
