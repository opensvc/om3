package commands

import (
	"opensvc.com/opensvc/core/nodeaction"
	"opensvc.com/opensvc/core/object"
)

type (
	CmdNodeComplianceAuto struct {
		OptsGlobal
		Moduleset string
		Module    string
		Force     bool
		Attach    bool
	}
)

func (t *CmdNodeComplianceAuto) Run() error {
	return nodeaction.New(
		nodeaction.WithLocal(t.Local),
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Format),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),
		nodeaction.WithRemoteAction("compliance auto"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"format":    t.Format,
			"force":     t.Force,
			"module":    t.Module,
			"moduleset": t.Moduleset,
			"attach":    t.Attach,
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
			run := comp.NewRun()
			run.SetModulesetsExpr(t.Moduleset)
			run.SetModulesExpr(t.Module)
			run.SetForce(t.Force)
			run.SetAttach(t.Attach)
			err = run.Auto()
			return run, err
		}),
	).Do()
}
