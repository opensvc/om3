package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/clientcontext"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/api"
)

type (
	CmdObjectPrintConfig struct {
		OptsGlobal
		Eval        bool
		Impersonate string
	}
)

type result map[string]rawconfig.T

func (t *CmdObjectPrintConfig) extract(selector string, c *client.T) (result, error) {
	data := make(result)
	paths, err := objectselector.NewSelection(
		selector,
		objectselector.SelectionWithLocal(true),
	).Expand()
	if err != nil {
		return data, err
	}
	for _, p := range paths {
		if d, err := t.extractOne(p, c); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s", p, err)
		} else {
			data[p.String()] = d
		}
	}
	return data, nil
}

func (t *CmdObjectPrintConfig) extractOne(p path.T, c *client.T) (rawconfig.T, error) {
	if data, err := t.extractFromDaemon(p, c); err == nil {
		return data, nil
	}
	if clientcontext.IsSet() {
		return rawconfig.T{}, fmt.Errorf("can not fetch from daemon")
	}
	return t.extractLocal(p)
}

func (t *CmdObjectPrintConfig) extractLocal(p path.T) (rawconfig.T, error) {
	obj, err := object.NewConfigurer(p)
	if err != nil {
		return rawconfig.T{}, err
	}
	if t.Eval {
		if t.Impersonate != "" {
			return obj.EvalConfigAs(t.Impersonate)
		}
		return obj.EvalConfig()
	}
	return obj.PrintConfig()
}

func (t *CmdObjectPrintConfig) extractFromDaemon(p path.T, c *client.T) (rawconfig.T, error) {
	params := api.GetObjectConfigParams{
		Path:        p.String(),
		Evaluate:    &t.Eval,
		Impersonate: &t.Impersonate,
	}
	resp, err := c.GetObjectConfigWithResponse(context.Background(), &params)
	if err != nil {
		return rawconfig.T{}, err
	} else if resp.StatusCode() != http.StatusOK {
		return rawconfig.T{}, fmt.Errorf("unexpected get object config status %s", resp.Status())
	}
	data := rawconfig.T{}
	if resp.JSON200 == nil {
		panic("response json is nil")
	} else if b, err := json.Marshal(resp.JSON200.Data); err != nil {
		return rawconfig.T{}, err
	} else if err := json.Unmarshal(b, &data); err != nil {
		return rawconfig.T{}, err
	} else {
		return data, nil
	}
}

func parseRoutedResponse(b []byte) (rawconfig.T, error) {
	type routedResponse struct {
		Nodes  map[string]rawconfig.T
		Status int
	}
	d := routedResponse{}
	err := json.Unmarshal(b, &d)
	if err != nil {
		return rawconfig.T{}, err
	}
	for _, cfg := range d.Nodes {
		return cfg, nil
	}
	return rawconfig.T{}, fmt.Errorf("no nodes in response")
}

func (t *CmdObjectPrintConfig) Run(selector, kind string) error {
	var (
		c    *client.T
		data result
		err  error
	)
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	if c, err = client.New(client.WithURL(t.Server)); err != nil {
		return err
	}
	if data, err = t.extract(mergedSelector, c); err != nil {
		return err
	}
	if len(data) == 0 {
		return fmt.Errorf("no match")
	}
	var render func() string
	if _, err := path.Parse(selector); err == nil {
		// single object selection
		render = func() string {
			d, _ := data[selector]
			return d.Render()
		}
		output.Renderer{
			Output:        t.Output,
			Color:         t.Color,
			Data:          data[selector].Data,
			HumanRenderer: render,
			Colorize:      rawconfig.Colorize,
		}.Print()
	} else {
		render = func() string {
			s := ""
			for p, d := range data {
				s += "#\n"
				s += "# path: " + p + "\n"
				s += "#\n"
				s += strings.Repeat("#", 78) + "\n"
				s += d.Render()
			}
			return s
		}
		output.Renderer{
			Output:        t.Output,
			Color:         t.Color,
			Data:          data,
			HumanRenderer: render,
			Colorize:      rawconfig.Colorize,
		}.Print()
	}
	return nil
}
