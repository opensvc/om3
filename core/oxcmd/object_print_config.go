package oxcmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/core/output"
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

func (t *CmdObjectPrintConfig) extract(selector string) (result, error) {
	data := make(result)
	c, err := client.New()
	if err != nil {
		return data, err
	}
	paths, err := objectselector.New(
		selector,
		objectselector.WithClient(c),
	).Expand()
	if err != nil {
		return data, err
	}
	for _, p := range paths {
		if d, err := t.extractFromDaemon(p, c); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", p, err)
		} else {
			data[p.String()] = d
		}
	}
	return data, nil
}

func (t *CmdObjectPrintConfig) extractFromDaemon(p naming.Path, c *client.T) (rawconfig.T, error) {
	params := api.GetObjectConfigParams{
		Evaluate:    &t.Eval,
		Impersonate: &t.Impersonate,
	}
	data := rawconfig.T{}
	resp, err := c.GetObjectConfigWithResponse(context.Background(), p.Namespace, p.Kind, p.Name, &params)

	if err != nil {
		return data, err
	} else if resp.StatusCode() != http.StatusOK {
		return data, fmt.Errorf("get object config: %s", resp.Status())
	}

	if b, err := json.Marshal(resp.JSON200.Data); err != nil {
		return data, err
	} else if err := json.Unmarshal(b, &data); err != nil {
		return data, err
	}

	return data, nil
}

func (t *CmdObjectPrintConfig) Run(selector, kind string) error {
	var (
		data result
		err  error
	)
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	if data, err = t.extract(mergedSelector); err != nil {
		return err
	}
	if len(data) == 0 {
		return fmt.Errorf("no match")
	}
	var render func() string
	if _, err := naming.ParsePath(selector); err == nil {
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
