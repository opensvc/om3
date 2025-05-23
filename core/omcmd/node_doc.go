package omcmd

import (
	"github.com/opensvc/om3/core/nodeaction"
	"github.com/opensvc/om3/core/object"
)

type (
	CmdNodeDoc struct {
		Color   string
		Output  string
		Keyword string
		Driver  string
		Depth   int
	}
)

func (t *CmdNodeDoc) Run() error {
	return nodeaction.New(
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
		nodeaction.WithLocalFunc(func() (interface{}, error) {
			n, err := object.NewNode(
				object.WithVolatile(true),
				object.WithConfigFile(""),
				object.WithClusterConfigFile(""),
			)
			if err != nil {
				return nil, err
			}
			return n.Doc(t.Driver, t.Keyword, t.Depth)
		}),
	).Do()
}
