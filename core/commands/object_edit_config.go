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
	"opensvc.com/opensvc/core/clientcontext"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/xconfig"
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
		Short:   "edit selected object and instance configuration",
		Aliases: []string{"confi", "conf", "con", "co", "c", "cf", "cfg"},
		Run: func(cmd *cobra.Command, args []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdObjectEditConfig) do(selector string, c *client.T) error {
	sel := object.NewSelection(selector)
	wc := clientcontext.IsSet()
	paths, err := sel.Expand()
	if err != nil {
		return err
	}
	for _, p := range paths {
		obj, err := object.NewConfigurerFromPath(p)
		if err != nil {
			return err
		}
		if !wc && object.Exists(p) {
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
	if errors.Is(err, xconfig.ErrEditPending) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err != nil {
		return err
	}
	return nil
}

func fetchConfig(p path.T, c *client.T) (s string, err error) {
	var (
		b []byte
	)
	handle := c.NewGetObjectConfig()
	handle.ObjectSelector = p.String()
	handle.Format = "ini"
	b, err = handle.Do()
	if err != nil {
		return "", err
	}
	if err = json.Unmarshal(b, &s); err != nil {
		return "", err
	}
	return s, nil
}

func pushConfig(p path.T, fName string, c *client.T) (err error) {
	var cfg *xconfig.T
	if cfg, err = xconfig.NewObject(fName); err != nil {
		return err
	}
	req := c.NewPostObjectCreate()
	req.ObjectSelector = p.String()
	req.Restore = true
	req.Data[p.String()] = cfg.Raw()
	_, err = req.Do()
	if err != nil {
		return err
	}
	return nil
}

func (t *CmdObjectEditConfig) doRemote(p path.T, c *client.T) error {
	var (
		err    error
		refSum []byte
		buff   string
		f      *os.File
	)
	if buff, err = fetchConfig(p, c); err != nil {
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
	if err = pushConfig(p, fName, c); err != nil {
		return err
	}
	return nil
}

func (t *CmdObjectEditConfig) run(selector *string, kind string) {
	var (
		c   *client.T
		err error
	)
	mergedSelector := mergeSelector(*selector, t.Global.ObjectSelector, kind, "")
	if c, err = client.New(client.WithURL(t.Global.Server)); err != nil {
		log.Error().Err(err).Msg("")
		os.Exit(1)
	}
	if err = t.do(mergedSelector, c); err != nil {
		log.Error().Err(err).Msg("")
		os.Exit(1)
	}
}
