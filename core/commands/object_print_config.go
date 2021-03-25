package commands

import (
	"encoding/json"
	"errors"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/config"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/output"
)

type (
	// CmdObjectPrintConfig is the cobra flag set of the print config command.
	CmdObjectPrintConfig struct {
		flagSetGlobal
		flagSetObject
		flagSetAction
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectPrintConfig) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	t.flagSetGlobal.init(cmd)
	t.flagSetObject.init(cmd)
	t.flagSetAction.init(cmd)
}

func (t *CmdObjectPrintConfig) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Print selected service and instance configuration",
		Run: func(cmd *cobra.Command, args []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdObjectPrintConfig) extract(selector string, c *client.T) ([]config.Raw, error) {
	if data, err := t.extractFromDaemon(selector, c); err == nil {
		return data, nil
	}
	if client.WantContext() {
		return []config.Raw{}, errors.New("can not fetch from daemon")
	}
	return t.extractLocal(selector)
}

func (t *CmdObjectPrintConfig) extractLocal(selector string) ([]config.Raw, error) {
	data := make([]config.Raw, 0)
	sel := object.NewSelection(selector).SetLocal(true)
	for _, path := range sel.Expand() {
		obj := path.NewConfigurer()
		c := obj.Config()
		if c == nil {
			log.Error().Str("path", path.String()).Msg("no configuration")
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
	data := make([]config.Raw, 1)
	handle := c.NewGetObjectConfig()
	handle.ObjectSelector = selector
	b, err = handle.Do()
	if err != nil {
		log.Error().Err(err).Msg("")
		return data, err
	}
	err = json.Unmarshal(b, &data[0])
	if err != nil {
		return data, err
	}
	return data, nil
}

func (t *CmdObjectPrintConfig) run(selector *string, kind string) {
	var (
		c    *client.T
		data []config.Raw
		err  error
	)
	mergedSelector := mergeSelector(*selector, t.ObjectSelector, kind, "")
	if c, err = client.New(client.URL(t.Server)); err != nil {
		log.Error().Err(err).Msg("")
		os.Exit(1)
	}
	if data, err = t.extract(mergedSelector, c); err != nil {
		log.Error().Err(err).Msg("")
		os.Exit(1)
	}
	output.Renderer{
		Format: t.Format,
		Color:  t.Color,
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
