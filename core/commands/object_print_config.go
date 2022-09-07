package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/clientcontext"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/objectselector"
	"opensvc.com/opensvc/core/output"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/rawconfig"
)

type (
	// CmdObjectPrintConfig is the cobra flag set of the print config command.
	CmdObjectPrintConfig struct {
		Command *cobra.Command
		OptsGlobal
		Eval        bool   `flag:"eval"`
		Impersonate string `flag:"impersonate"`
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectPrintConfig) Init(kind string, parent *cobra.Command, selector *string) {
	t.Command = t.cmd(kind, selector)
	parent.AddCommand(t.Command)
	flag.Install(t.Command, t)
}

func (t *CmdObjectPrintConfig) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:     "config",
		Short:   "Print selected object and instance configuration",
		Aliases: []string{"confi", "conf", "con", "co", "c", "cf", "cfg"},
		Run: func(cmd *cobra.Command, args []string) {
			t.run(selector, kind)
		},
	}
}

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
		var err error
		data[p.String()], err = t.extractOne(p, c)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s", p, err)
		}
	}
	return data, nil
}

func (t *CmdObjectPrintConfig) extractOne(p path.T, c *client.T) (rawconfig.T, error) {
	if data, err := t.extractFromDaemon(p, c); err == nil {
		return data, nil
	}
	if clientcontext.IsSet() {
		return rawconfig.T{}, errors.New("can not fetch from daemon")
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
	var (
		err error
		b   []byte
	)
	handle := c.NewGetObjectConfig()
	handle.ObjectSelector = p.String()
	handle.Evaluate = t.Eval
	handle.Impersonate = t.Impersonate
	handle.SetNode(t.NodeSelector)
	b, err = handle.Do()
	if err != nil {
		return rawconfig.T{}, err
	}
	if data, err := parseRoutedResponse(b); err == nil {
		return data, nil
	}
	data := rawconfig.T{}
	if err := json.Unmarshal(b, &data); err == nil {
		return data, nil
	} else {
		return rawconfig.T{}, err
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

func (t *CmdObjectPrintConfig) run(selector *string, kind string) {
	var (
		c    *client.T
		data result
		err  error
	)
	mergedSelector := mergeSelector(*selector, t.ObjectSelector, kind, "")
	if c, err = client.New(client.WithURL(t.Server)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if data, err = t.extract(mergedSelector, c); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if len(data) == 0 {
		fmt.Fprintln(os.Stderr, "no match")
		os.Exit(1)
	}
	var render func() string
	if p, err := path.Parse(*selector); err == nil {
		render = func() string {
			d, _ := data[*selector].Data.Get("data")
			o, err := object.NewConfigurer(
				p,
				object.WithVolatile(true),
				object.WithConfigData(d),
			)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			n, _ := o.PrintConfig()
			return n.Render()
		}
		output.Renderer{
			Format:        t.Format,
			Color:         t.Color,
			Data:          data[*selector].Data,
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
			Format:        t.Format,
			Color:         t.Color,
			Data:          data,
			HumanRenderer: render,
			Colorize:      rawconfig.Colorize,
		}.Print()
	}
}
