package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/config"
	"opensvc.com/opensvc/core/client"
	clientcontext "opensvc.com/opensvc/core/client/context"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/output"
	"opensvc.com/opensvc/core/path"
)

type (
	// CmdObjectPrintConfig is the cobra flag set of the print config command.
	CmdObjectPrintConfig struct {
		object.OptsPrintConfig
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectPrintConfig) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
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

type result map[string]config.Raw

func (t *CmdObjectPrintConfig) extract(selector string, c *client.T) (result, error) {
	paths := object.NewSelection(
		selector,
		object.SelectionWithLocal(true),
	).Expand()
	data := make(result)
	for _, p := range paths {
		var err error
		data[p.String()], err = t.extractOne(p, c)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s", p, err)
		}
	}
	return data, nil
}

func (t *CmdObjectPrintConfig) extractOne(p path.T, c *client.T) (config.Raw, error) {
	if data, err := t.extractFromDaemon(p, c); err == nil {
		return data, nil
	}
	if clientcontext.IsSet() {
		return config.Raw{}, errors.New("can not fetch from daemon")
	}
	return t.extractLocal(p)
}

func (t *CmdObjectPrintConfig) extractLocal(p path.T) (config.Raw, error) {
	obj := object.NewConfigurerFromPath(p)
	c := obj.Config()
	if c == nil {
		return config.Raw{}, fmt.Errorf("path %s: no configuration")
	}
	return c.Raw(), nil
}

func (t *CmdObjectPrintConfig) extractFromDaemon(p path.T, c *client.T) (config.Raw, error) {
	var (
		err error
		b   []byte
	)
	handle := c.NewGetObjectConfig()
	handle.ObjectSelector = p.String()
	handle.Evaluate = t.Eval
	handle.Impersonate = t.Impersonate
	handle.SetNode(t.Global.NodeSelector)
	b, err = handle.Do()
	if err != nil {
		return config.Raw{}, err
	}
	if data, err := parseRoutedResponse(b); err == nil {
		return data, nil
	}
	data := config.Raw{}
	if err := json.Unmarshal(b, &data); err == nil {
		return data, nil
	} else {
		return config.Raw{}, err
	}
}

func parseRoutedResponse(b []byte) (config.Raw, error) {
	type routedResponse struct {
		Nodes  map[string]config.Raw
		Status int
	}
	d := routedResponse{}
	err := json.Unmarshal(b, &d)
	if err != nil {
		return config.Raw{}, err
	}
	for _, config := range d.Nodes {
		return config, nil
	}
	return config.Raw{}, fmt.Errorf("path %s: not found in response")
}

func (t *CmdObjectPrintConfig) run(selector *string, kind string) {
	var (
		c    *client.T
		data result
		err  error
	)
	mergedSelector := mergeSelector(*selector, t.Global.ObjectSelector, kind, "")
	if c, err = client.New(client.WithURL(t.Global.Server)); err != nil {
		log.Error().Err(err).Msg("")
		os.Exit(1)
	}
	if data, err = t.extract(mergedSelector, c); err != nil {
		log.Error().Err(err).Msg("")
		os.Exit(1)
	}
	if len(data) == 0 {
		fmt.Fprintln(os.Stderr, "no match")
		os.Exit(1)
	}
	var render func() string
	if _, err := path.Parse(*selector); err == nil {
		render = func() string {
			return data[*selector].Render()
		}
		output.Renderer{
			Format:        t.Global.Format,
			Color:         t.Global.Color,
			Data:          data[*selector],
			HumanRenderer: render,
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
			Format:        t.Global.Format,
			Color:         t.Global.Color,
			Data:          data,
			HumanRenderer: render,
		}.Print()
	}
}
