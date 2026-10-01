package omcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/configkeywords"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/objectselector"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	CmdObjectConfigGet struct {
		OptsGlobal
		Local       bool
		Eval        bool
		Impersonate string
		Keywords    []string
	}
)

func (t *CmdObjectConfigGet) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	var (
		l   api.KeywordItems
		err error
	)
	if t.Local {
		l, err = t.localItems(mergedSelector)
	} else if l, err = t.daemonItems(mergedSelector); client.IsDaemonDown(err) {
		// om runs where the configurations are: with the daemon down, it
		// reads them itself, and answers what the daemon would have.
		l, err = t.localItems(mergedSelector)
	}
	if err != nil {
		return err
	}
	return t.render(l)
}

func (t *CmdObjectConfigGet) options() configkeywords.Options {
	return configkeywords.Options{
		Keywords:    t.Keywords,
		Evaluate:    t.Eval,
		Impersonate: t.Impersonate,
	}
}

// localItems answers the keywords from the configuration files of this node,
// as the daemon does.
func (t *CmdObjectConfigGet) localItems(mergedSelector string) (api.KeywordItems, error) {
	sel := objectselector.New(mergedSelector)
	var (
		paths naming.Paths
		err   error
	)
	if t.IgnoreNotFound {
		paths, err = sel.ExpandRelaxed()
	} else {
		paths, err = sel.MustExpand()
	}
	if err != nil {
		return nil, err
	}
	l := make(api.KeywordItems, 0)
	for _, p := range paths {
		items, err := configkeywords.Object(p, t.options())
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		l = append(l, items...)
	}
	return l, nil
}

// daemonItems asks the daemon for the keywords.
func (t *CmdObjectConfigGet) daemonItems(mergedSelector string) (api.KeywordItems, error) {
	c, err := client.New()
	if err != nil {
		return nil, err
	}
	sel := objectselector.New(mergedSelector, objectselector.WithClient(c))
	var paths naming.Paths
	if t.IgnoreNotFound {
		paths, err = sel.ExpandRelaxed()
	} else {
		paths, err = sel.MustExpand()
	}
	if err != nil {
		return nil, err
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
			return nil, err
		}
		switch {
		case response.JSON200 != nil:
			l = append(l, response.JSON200.Items...)
		case response.JSON400 != nil:
			return nil, fmt.Errorf("%s: %s", p, *response.JSON400)
		case response.JSON401 != nil:
			return nil, fmt.Errorf("%s: %s", p, *response.JSON401)
		case response.JSON403 != nil:
			return nil, fmt.Errorf("%s: %s", p, *response.JSON403)
		case response.JSON500 != nil:
			return nil, fmt.Errorf("%s: %s", p, *response.JSON500)
		default:
			return nil, fmt.Errorf("%s: unexpected response: %s", p, response.Status())
		}
	}
	return l, nil
}

func (t *CmdObjectConfigGet) render(l api.KeywordItems) error {
	var defaultOutput string
	// A pattern answers a list whatever the number of keys it matched: a
	// bare value would not say which key it is the value of.
	listed := len(l) > 1 || configkeywords.HasPattern(t.Keywords)
	if t.Eval {
		if hasEvalError(l) {
			defaultOutput = "tab=OBJECT:object,KEYWORD:keyword,VALUE:value,EVALUATED:evaluated_text,EVALUATED_AS:evaluated_as,ERROR:error"
		} else if listed {
			defaultOutput = "tab=OBJECT:object,KEYWORD:keyword,VALUE:value,EVALUATED:evaluated_text,EVALUATED_AS:evaluated_as"
		} else {
			defaultOutput = "tab=evaluated_text"
		}
	} else {
		if listed {
			defaultOutput = "tab=OBJECT:object,KEYWORD:keyword,VALUE:value"
		} else {
			defaultOutput = "tab=value"
		}
	}

	return output.Renderer{
		DefaultOutput: defaultOutput,
		Output:        t.Output,
		Sort:          t.Sort,
		Color:         t.Color,
		Data:          api.KeywordList{Items: api.WithEvaluatedText(l), Kind: "KeywordList"},
		Colorize:      rawconfig.Colorize,
	}.Print()
}
