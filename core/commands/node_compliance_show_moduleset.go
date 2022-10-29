package commands

import (
	"opensvc.com/opensvc/core/nodeaction"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/util/xstrings"
)

type (
	CmdNodeComplianceShowModuleset struct {
		OptsGlobal
		Moduleset string
	}
)

func (t *CmdNodeComplianceShowModuleset) Run() error {
	return nodeaction.New(
		nodeaction.WithLocal(t.Local),
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Format),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),
		nodeaction.WithRemoteAction("compliance show moduleset"),
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
			modsets := xstrings.Split(t.Moduleset, ",")
			data, err := comp.GetData(modsets)
			if err != nil {
				return nil, err
			}
			tree := data.ModulesetsTree()
			return tree, nil
		}),
	).Do()
}
