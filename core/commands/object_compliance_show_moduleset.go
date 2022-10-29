package commands

import (
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/objectaction"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/util/xstrings"
)

type (
	CmdObjectComplianceShowModuleset struct {
		OptsGlobal
		Moduleset string
	}
)

func (t *CmdObjectComplianceShowModuleset) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Local),
		objectaction.WithColor(t.Color),
		objectaction.WithFormat(t.Format),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithServer(t.Server),
		objectaction.WithRemoteAction("compliance show moduleset"),
		objectaction.WithRemoteOptions(map[string]interface{}{
			"format":    t.Format,
			"moduleset": t.Moduleset,
		}),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			if o, err := object.NewSvc(p); err != nil {
				return nil, err
			} else {
				comp, err := o.NewCompliance()
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
			}
		}),
	).Do()
}
