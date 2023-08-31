package commands

import (
	"github.com/opensvc/om3/core/nodeaction"
	"github.com/opensvc/om3/core/object"
)

type (
	CmdNodeRegister struct {
		OptsGlobal
		User     string
		Password string
		App      string
	}
)

func (t *CmdNodeRegister) Run() error {
	return nodeaction.New(
		nodeaction.WithLocal(t.Local),
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),
		nodeaction.WithRemoteAction("register"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"format": t.Output,
		}),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			n, err := object.NewNode()
			if err != nil {
				return nil, err
			}
			return nil, n.Register(t.User, t.Password, t.App)
		}),
	).Do()
}
