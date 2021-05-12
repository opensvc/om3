package commands

import (
	"encoding/json"
	"errors"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/config"
	"opensvc.com/opensvc/core/client"
	clientcontext "opensvc.com/opensvc/core/client/context"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/output"
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

func (t *CmdObjectPrintConfig) extract(selector string, c *client.T) ([]config.Raw, error) {
	if data, err := t.extractFromDaemon(selector, c); err == nil {
		return data, nil
	}
	if clientcontext.IsSet() {
		return []config.Raw{}, errors.New("can not fetch from daemon")
	}
	return t.extractLocal(selector)
}

func (t *CmdObjectPrintConfig) extractLocal(selector string) ([]config.Raw, error) {
	data := make([]config.Raw, 0)
	sel := object.NewSelection(
		selector,
		object.SelectionWithLocal(true),
	)
	for _, p := range sel.Expand() {
		obj := object.NewConfigurerFromPath(p)
		c := obj.Config()
		if c == nil {
			log.Error().Stringer("path", p).Msg("no configuration")
			continue
		}
		data = append(data, c.Raw())
	}
	return data, nil
}

func (t *CmdObjectPrintConfig) extractFromDaemon(selector string, c *client.T) ([]config.Raw, error) {
	var (
		err error
		b   []byte
	)
	handle := c.NewGetObjectConfig()
	handle.ObjectSelector = selector
	handle.Evaluate = t.Eval
	handle.Impersonate = t.Impersonate
	handle.SetNode(t.Global.NodeSelector)
	b, err = handle.Do()
	if err != nil {
		log.Error().Err(err).Msg("")
		data := make([]config.Raw, 0)
		return data, err
	}
	data := make([]config.Raw, 1)
	if data, err := parseRoutedResponse(b); err == nil {
		return data, nil
	}
	err = json.Unmarshal(b, &data[0])
	if err != nil {
		return data, err
	}
	return data, nil
}

func parseRoutedResponse(b []byte) ([]config.Raw, error) {
	data := make([]config.Raw, 0)
	type routedResponse struct {
		Nodes  map[string]config.Raw
		Status int
	}
	d := routedResponse{}
	err := json.Unmarshal(b, &d)
	if err != nil {
		return data, err
	}
	for _, config := range d.Nodes {
		data = append(data, config)
	}
	return data, nil
}

func (t *CmdObjectPrintConfig) run(selector *string, kind string) {
	var (
		c    *client.T
		data []config.Raw
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
	output.Renderer{
		Format: t.Global.Format,
		Color:  t.Global.Color,
		Data:   data,
		HumanRenderer: func() string {
			s := ""
			for _, d := range data {
				s += d.Render()
			}
			return s
		},
	}.Print()
}
