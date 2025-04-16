package omcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/commoncmd"
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
	CmdObjectConfigEval struct {
		OptsGlobal
		Keywords    []string
		Impersonate string
	}
)

func (t *CmdObjectConfigEval) Run(selector, kind string) error {
	mergedSelector := commoncmd.MergeSelector(selector, t.ObjectSelector, kind, "")
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
		params := api.GetObjectConfigParams{}
		params.Kw = &t.Keywords
		v := true
		params.Evaluate = &v
		if t.Impersonate != "" {
			params.Impersonate = &t.Impersonate
		}
		response, err := c.GetObjectConfigWithResponse(context.Background(), p.Namespace, p.Kind, p.Name, &params)
		if err != nil {
			return err
		}
		switch response.StatusCode() {
		case 200:
			l = append(l, response.JSON200.Items...)
		case 400:
			return fmt.Errorf("%s: %s", p, *response.JSON400)
		case 401:
			return fmt.Errorf("%s: %s", p, *response.JSON401)
		case 403:
			return fmt.Errorf("%s: %s", p, *response.JSON403)
		case 500:
			return fmt.Errorf("%s: %s", p, *response.JSON500)
		default:
			return fmt.Errorf("%s: unexpected response: %s", p, response.Status())
		}
	}

	defaultOutput := "tab=data.value"
	if len(l) > 1 {
		defaultOutput = "tab=OBJECT:meta.object,NODE:meta.node,KEYWORD:meta.keyword,VALUE:data.value"
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

func (t *CmdObjectConfigEval) doObjectAction(mergedSelector string) error {
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
				return c.EvalAs(kw, t.Impersonate)
			}
			return nil, nil
		}),
	).Do()
}
