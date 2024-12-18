package omcmd

import (
	"github.com/opensvc/om3/core/nodeaction"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/util/xstrings"
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
		nodeaction.WithLocal(t.Local),
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
		nodeaction.WithLocalFunc(func() (interface{}, error) {
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
