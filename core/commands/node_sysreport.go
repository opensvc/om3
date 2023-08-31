package commands

import (
	"github.com/opensvc/om3/core/nodeaction"
	"github.com/opensvc/om3/core/object"
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
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),
		nodeaction.WithRemoteAction("sysreport"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"format": t.Output,
			"force":  t.Force,
		}),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			n, err := object.NewNode()
			if err != nil {
				return nil, err
			}
			if t.Force {
				err := n.ForceSysreport()
				return nil, err
			} else {
				err := n.Sysreport()
				return nil, err
			}
		}),
	).Do()
}
