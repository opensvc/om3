package commands

import (
	"io/ioutil"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/util/editor"
)

type (
	// CmdObjectEditConfig is the cobra flag set of the print config command.
	CmdObjectEditConfig struct {
		Global     object.OptsGlobal
		EditConfig object.OptsEditConfig
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectEditConfig) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *CmdObjectEditConfig) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:     "config",
		Short:   "Edit selected object and instance configuration",
		Aliases: []string{"confi", "conf", "con", "co", "c", "cf", "cfg"},
		Run: func(cmd *cobra.Command, args []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdObjectEditConfig) do(selector string, c *client.T) error {
	sel := object.NewSelection(selector)
	for _, p := range sel.Expand() {
		obj := object.NewConfigurerFromPath(p)
		if obj.Exists() {
			if err := t.doLocal(obj, c); err != nil {
				return err
			}
			continue
		}
		if err := t.doRemote(p, c); err != nil {
			return err
		}
	}
	return nil
}

func (t *CmdObjectEditConfig) doLocal(obj object.Configurer, c *client.T) error {
	err := obj.EditConfig(t.EditConfig)
	if err != nil {
		return err
	}
	return nil
}

func (t *CmdObjectEditConfig) doRemote(p path.T, c *client.T) error {
	var (
		err error
		b   []byte
	)
	handle := c.NewGetObjectConfig()
	handle.ObjectSelector = p.String()
	handle.Format = "raw"
	b, err = handle.Do()
	if err != nil {
		return err
	}
	file, err := ioutil.TempFile("", ".opensvc.edit.config.*.swp")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(b); err != nil {
		return err
	}
	if err = editor.Edit(file.Name()); err != nil {
		return err
	}
	// TODO: send
	return nil
}

func (t *CmdObjectEditConfig) run(selector *string, kind string) {
	var (
		c   *client.T
		err error
	)
	mergedSelector := mergeSelector(*selector, t.Global.ObjectSelector, kind, "")
	if c, err = client.New(client.URL(t.Global.Server)); err != nil {
		log.Error().Err(err).Msg("")
		os.Exit(1)
	}
	if err = t.do(mergedSelector, c); err != nil {
		log.Error().Err(err).Msg("")
		os.Exit(1)
	}
}
