package omcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectaction"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/key"
)

type (
	CmdObjectGet struct {
		OptsGlobal
		Eval        bool
		Impersonate string
		Keywords    []string
	}
)

func (t *CmdObjectGet) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	if t.Local {
		return t.doObjectAction(mergedSelector)
	}
	c, err := client.New()
	if err != nil {
		return err
	}
	sel := objectselector.New(mergedSelector, objectselector.WithClient(c))
	paths, err := sel.MustExpand()
	if err != nil {
		return err
	}
	l := make(api.KeywordItems, 0)
	for _, p := range paths {
		params := api.GetObjectConfigGetParams{}
		params.Kw = &t.Keywords
		if t.Eval {
			v := true
			params.Evaluate = &v
		}
		if t.Impersonate != "" {
			params.Impersonate = &t.Impersonate
		}
		response, err := c.GetObjectConfigGetWithResponse(context.Background(), p.Namespace, p.Kind, p.Name, &params)
		if err != nil {
			return err
		}
		switch {
		case response.JSON200 != nil:
			l = append(l, response.JSON200.Items...)
		case response.JSON400 != nil:
			return fmt.Errorf("%s: %s", p, *response.JSON400)
		case response.JSON401 != nil:
			return fmt.Errorf("%s: %s", p, *response.JSON401)
		case response.JSON403 != nil:
			return fmt.Errorf("%s: %s", p, *response.JSON403)
		case response.JSON500 != nil:
			return fmt.Errorf("%s: %s", p, *response.JSON500)
		default:
			return fmt.Errorf("%s: unexpected response: %s", p, response.Status())
		}
	}
	defaultOutput := "tab=OBJECT:meta.object,KEYWORD:meta.keyword,VALUE:data.value"
	if t.Eval {
		defaultOutput += ",EVALUATED_AS:meta.evaluated_as"
	}
	output.Renderer{
		DefaultOutput: defaultOutput,
		Output:        t.Output,
		Color:         t.Color,
		Data:          api.KeywordList{Items: l, Kind: "KeywordList"},
		Colorize:      rawconfig.Colorize,
	}.Print()

	return nil
}

func (t *CmdObjectGet) doObjectAction(mergedSelector string) error {
	return objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Local),
		objectaction.WithColor(t.Color),
		objectaction.WithOutput(t.Output),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithLocalFunc(func(ctx context.Context, p naming.Path) (interface{}, error) {
			c, err := object.NewConfigurer(p)
			if err != nil {
				return nil, err
			}
			for _, s := range t.Keywords {
				kw := key.Parse(s)
				if t.Eval {
					if t.Impersonate != "" {
						return c.EvalAs(kw, t.Impersonate)
					} else {
						return c.Eval(kw)
					}
				} else {
					return c.Get(kw)
				}
			}
			return nil, nil
		}),
	).Do()
}
