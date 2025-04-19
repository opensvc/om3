package oxcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/api"
)

type (
	CmdObjectConfigGet struct {
		OptsGlobal
		Eval        bool
		Impersonate string
		Keywords    []string
	}
)

func (t *CmdObjectConfigGet) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
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
		if len(t.Keywords) > 0 {
			params.Kw = &t.Keywords
		}
		if t.Eval {
			v := true
			params.Evaluate = &v
		}
		if t.Impersonate != "" {
			params.Impersonate = &t.Impersonate
		}
		response, err := c.GetObjectConfigWithResponse(context.Background(), p.Namespace, p.Kind, p.Name, &params)
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

	var defaultOutput string
	if t.Eval {
		if len(l) > 1 {
			defaultOutput = "tab=OBJECT:object,KEYWORD:keyword,VALUE:value,EVALUATED:evaluated,EVALUATED_AS:evaluated_as"
		} else {
			defaultOutput = "tab=evaluated"
		}
	} else {
		if len(l) > 1 {
			defaultOutput = "tab=OBJECT:object,KEYWORD:keyword,VALUE:value"
		} else {
			defaultOutput = "tab=value"
		}
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
