package oxcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/api"
)

type (
	CmdNodeEval struct {
		OptsGlobal
		OptsLock
		Keywords     []string
		Impersonate  string
		NodeSelector string
	}
)

func (t *CmdNodeEval) Run() error {
	c, err := client.New()
	if err != nil {
		return err
	}

	if t.NodeSelector == "" {
		t.NodeSelector = "*"
	}

	sel := nodeselector.New(t.NodeSelector)
	nodenames, err := sel.Expand()
	if err != nil {
		return err
	}

	l := make(api.KeywordItems, 0)
	for _, nodename := range nodenames {
		params := api.GetNodeConfigGetParams{}
		params.Kw = &t.Keywords
		v := true
		params.Evaluate = &v
		if t.Impersonate != "" {
			params.Impersonate = &t.Impersonate
		}
		response, err := c.GetNodeConfigGetWithResponse(context.Background(), nodename, &params)
		if err != nil {
			return err
		}
		switch {
		case response.JSON200 != nil:
			l = append(l, response.JSON200.Items...)
		case response.JSON400 != nil:
			return fmt.Errorf("%s: %s", nodename, *response.JSON400)
		case response.JSON401 != nil:
			return fmt.Errorf("%s: %s", nodename, *response.JSON401)
		case response.JSON403 != nil:
			return fmt.Errorf("%s: %s", nodename, *response.JSON403)
		case response.JSON500 != nil:
			return fmt.Errorf("%s: %s", nodename, *response.JSON500)
		default:
			return fmt.Errorf("%s: unexpected response: %s", nodename, response.Status())
		}
	}
	defaultOutput := "tab=NODE:meta.node,KEYWORD:meta.keyword,VALUE:data.value,EVALUATED_AS:meta.evaluated_as"
	output.Renderer{
		DefaultOutput: defaultOutput,
		Output:        t.Output,
		Color:         t.Color,
		Data:          api.KeywordList{Items: l, Kind: "KeywordList"},
		Items:         l,
		Colorize:      rawconfig.Colorize,
	}.Print()

	return nil
}
