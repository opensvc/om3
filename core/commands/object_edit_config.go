package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/util/editor"
	"opensvc.com/opensvc/util/file"
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
	wc := client.WantContext()
	for _, p := range sel.Expand() {
		obj := object.NewConfigurerFromPath(p)
		if !wc && obj.Exists() {
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
	if errors.Is(err, object.ErrEditConfigPending) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err != nil {
		return err
	}
	return nil
}

func (t *CmdObjectEditConfig) doRemote(p path.T, c *client.T) error {
	var (
		err       error
		b, refSum []byte
		buff      string
		f         *os.File
	)
	handle := c.NewGetObjectConfig()
	handle.ObjectSelector = p.String()
	handle.Format = "ini"
	b, err = handle.Do()
	if err != nil {
		return err
	}
	if err = json.Unmarshal(b, &buff); err != nil {
		return err
	}
	if f, err = ioutil.TempFile("", ".opensvc.edit.config.*"); err != nil {
		return err
	}
	fName := f.Name()
	defer os.Remove(fName)
	if _, err = f.Write([]byte(buff)); err != nil {
		return err
	}
	if refSum, err = file.MD5(fName); err != nil {
		return err
	}
	if err = editor.Edit(fName); err != nil {
		return err
	}
	if file.HaveSameMD5(refSum, fName) {
		fmt.Println("unchanged")
		return nil
	}
	if b, err = ioutil.ReadAll(f); err != nil {
		return err
	}
	//handle = c.NewPostObjectConfig()
	//handle.ObjectSelector = p.String()
	//handle.Format = "ini"
	//handle.Data = string(b)
	//_, err = handle.Do()
	//if err != nil {
	//	return err
	//}
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
