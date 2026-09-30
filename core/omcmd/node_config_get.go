package omcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/configkeywords"
	"github.com/opensvc/om3/v3/core/nodeselector"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/hostname"
)

type (
	CmdNodeConfigGet struct {
		OptsGlobal
		commoncmd.OptsLock
		Local        bool
		Eval         bool
		Impersonate  string
		Keywords     []string
		NodeSelector string
	}
)

func (t *CmdNodeConfigGet) Run() error {
	var (
		l   api.KeywordItems
		err error
	)
	if t.Local {
		l, err = t.localItems()
	} else if l, err = t.daemonItems(); client.IsDaemonDown(err) && t.isLocalNodeOnly() {
		// om runs where the configuration is: with the daemon down, it
		// reads it itself, and answers what the daemon would have. The
		// configuration of another node is the daemon's to fetch.
		l, err = t.localItems()
	}
	if err != nil {
		return err
	}
	return t.render(l)
}

// isLocalNodeOnly says the selection is the local node alone.
func (t *CmdNodeConfigGet) isLocalNodeOnly() bool {
	switch t.NodeSelector {
	case "", hostname.Hostname(), "localhost":
		return true
	default:
		return false
	}
}

// localItems answers the keywords from the configuration files of this node,
// as the daemon does.
func (t *CmdNodeConfigGet) localItems() (api.KeywordItems, error) {
	return configkeywords.Node(hostname.Hostname(), configkeywords.Options{
		Keywords:    t.Keywords,
		Evaluate:    t.Eval,
		Impersonate: t.Impersonate,
	})
}

// daemonItems asks the daemon for the keywords of the selected nodes.
func (t *CmdNodeConfigGet) daemonItems() (api.KeywordItems, error) {
	c, err := client.New()
	if err != nil {
		return nil, err
	}

	nodenames := []string{hostname.Hostname()}
	if t.NodeSelector != "" {
		sel := nodeselector.New(t.NodeSelector)
		if l, err := sel.Expand(); err != nil {
			return nil, err
		} else {
			nodenames = l
		}
	}
	l := make(api.KeywordItems, 0)
	for _, nodename := range nodenames {
		params := api.GetNodeConfigParams{}
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
		response, err := c.GetNodeConfigWithResponse(context.Background(), nodename, &params)
		if err != nil {
			return nil, err
		}
		switch {
		case response.JSON200 != nil:
			l = append(l, response.JSON200.Items...)
		case response.JSON400 != nil:
			return nil, fmt.Errorf("%s: %s", nodename, *response.JSON400)
		case response.JSON401 != nil:
			return nil, fmt.Errorf("%s: %s", nodename, *response.JSON401)
		case response.JSON403 != nil:
			return nil, fmt.Errorf("%s: %s", nodename, *response.JSON403)
		case response.JSON500 != nil:
			return nil, fmt.Errorf("%s: %s", nodename, *response.JSON500)
		default:
			return nil, fmt.Errorf("%s: unexpected response: %s", nodename, response.Status())
		}
	}
	return l, nil
}

func (t *CmdNodeConfigGet) render(l api.KeywordItems) error {
	var defaultOutput string
	if t.Eval {
		if hasEvalError(l) {
			defaultOutput = "tab=NODE:node,KEYWORD:keyword,VALUE:value,EVALUATED:evaluated_text,EVALUATED_AS:evaluated_as,ERROR:error"
		} else if len(l) > 1 {
			defaultOutput = "tab=NODE:node,KEYWORD:keyword,VALUE:value,EVALUATED:evaluated_text,EVALUATED_AS:evaluated_as"
		} else {
			defaultOutput = "tab=evaluated_text"
		}
	} else {
		if len(l) > 1 {
			defaultOutput = "tab=NODE:node,KEYWORD:keyword,VALUE:value"
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
