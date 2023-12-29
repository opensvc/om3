package commands

import (
	"github.com/opensvc/om3/core/nodeaction"
	"github.com/opensvc/om3/core/object"
)

type (
	CmdNodeSysreport struct {
		OptsGlobal
		Force        bool
		NodeSelector string
	}
)

func (t *CmdNodeSysreport) Run() error {
	return nodeaction.New(
		nodeaction.WithLocal(t.Local),
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),
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
