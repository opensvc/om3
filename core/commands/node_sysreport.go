package commands

import (
	"opensvc.com/opensvc/core/nodeaction"
	"opensvc.com/opensvc/core/object"
)

type (
	CmdNodeSysreport struct {
		OptsGlobal
		Force bool
	}
)

func (t *CmdNodeSysreport) Run() error {
	return nodeaction.New(
		nodeaction.WithLocal(t.Local),
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Format),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),
		nodeaction.WithRemoteAction("sysreport"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"format": t.Format,
			"force":  t.Force,
		}),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			n, err := object.NewNode()
			if err != nil {
				return nil, err
			}
			if t.Force {
				return nil, n.ForceSysreport()
			} else {
				return nil, n.Sysreport()
			}
		}),
	).Do()
}
