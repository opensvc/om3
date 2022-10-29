package commands

import (
	"opensvc.com/opensvc/core/nodeaction"
	"opensvc.com/opensvc/core/object"
)

type (
	CmdNodeDoc struct {
		OptsGlobal
		Keyword string
		Driver  string
	}
)

func (t *CmdNodeDoc) Run() error {
	return nodeaction.New(
		nodeaction.WithFormat(t.Format),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),

		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithRemoteAction("node doc"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"kw":     t.Keyword,
			"driver": t.Driver,
		}),

		nodeaction.WithLocal(t.Local),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			n, err := object.NewNode()
			if err != nil {
				return nil, err
			}
			switch {
			case t.Driver != "":
				return n.DriverDoc(t.Driver)
			case t.Keyword != "":
				return n.KeywordDoc(t.Keyword)
			default:
				return "", nil
			}
		}),
	).Do()
}
