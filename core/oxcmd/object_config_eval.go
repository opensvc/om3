package oxcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/api"
)

type (
	CmdObjectConfigEval struct {
		OptsGlobal
		Keywords    []string
		Impersonate string
	}
)

func (t *CmdObjectConfigEval) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
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
		v := true
		params.Evaluate = &v
		if t.Impersonate != "" {
			params.Impersonate = &t.Impersonate
		}
		response, err := c.GetObjectConfigGetWithResponse(context.Background(), p.Namespace, p.Kind, p.Name, &params)
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
