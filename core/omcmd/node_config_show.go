package omcmd

import (
	"github.com/opensvc/om3/core/nodeaction"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/rawconfig"
)

type (
	CmdNodeConfigShow struct {
		OptsGlobal
		Eval         bool
		Impersonate  string
		NodeSelector string
		Sections     []string
	}
)

func (t *CmdNodeConfigShow) Run() error {
	return nodeaction.New(
		nodeaction.LocalFirst(),
		nodeaction.WithLocal(t.Local),
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
		nodeaction.WithLocalFunc(func() (interface{}, error) {
			n, err := object.NewNode()
			if err != nil {
				return nil, err
			}
			data := rawconfig.New()
			switch {
			case t.Eval:
				data, err = n.EvalConfigAs(t.Impersonate)
			default:
				data, err = n.RawConfig()
			}
			if err != nil {
				return nil, err
			}
			return data.Filter(t.Sections), nil
		}),
	).Do()
}
