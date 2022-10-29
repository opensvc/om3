package commands

import (
	"fmt"

	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/objectaction"
	"opensvc.com/opensvc/core/path"
)

type (
	CmdObjectDoc struct {
		OptsGlobal
		Keyword string
		Driver  string
	}
)

func (t *CmdObjectDoc) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Local),
		objectaction.WithColor(t.Color),
		objectaction.WithFormat(t.Format),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithRemoteAction("doc"),
		objectaction.WithRemoteOptions(map[string]interface{}{
			"kw":     t.Keyword,
			"driver": t.Driver,
		}),
		objectaction.WithLocalRun(func(p path.T) (interface{}, error) {
			o, err := object.New(p)
			if err != nil {
				return nil, err
			}
			c, ok := o.(object.Configurer)
			if !ok {
				return nil, fmt.Errorf("%s is not a configurer", o)
			}
			switch {
			case t.Driver != "":
				return c.DriverDoc(t.Driver)
			case t.Keyword != "":
				return c.KeywordDoc(t.Keyword)
			default:
				return "", nil
			}
		}),
	).Do()
}
