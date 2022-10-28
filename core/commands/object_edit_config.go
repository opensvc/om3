package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/clientcontext"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/objectselector"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/xconfig"
	"opensvc.com/opensvc/util/editor"
	"opensvc.com/opensvc/util/file"
)

type (
	// CmdObjectEditConfig is the cobra flag set of the print config command.
	CmdObjectEditConfig struct {
		OptsGlobal
		Discard bool `flag:"discard"`
		Recover bool `flag:"recover"`
	}
)

func (t *CmdObjectEditConfig) do(selector string, c *client.T) error {
	sel := objectselector.NewSelection(selector)
	wc := clientcontext.IsSet()
	paths, err := sel.Expand()
	if err != nil {
		return err
	}
	for _, p := range paths {
		obj, err := object.NewConfigurer(p)
		if err != nil {
			return err
		}
		if !wc && p.Exists() {
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
	var err error
	switch {
	case t.Discard:
		err = obj.DiscardAndEditConfig()
	case t.Recover:
		err = obj.RecoverAndEditConfig()
	default:
		err = obj.EditConfig()
	}
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
	if cfg, err = xconfig.NewObject("", fName); err != nil {
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

func (t *CmdObjectEditConfig) Run(selector, kind string) error {
	var (
		c   *client.T
		err error
	)
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	if c, err = client.New(client.WithURL(t.Server)); err != nil {
		return err
	}
	return t.do(mergedSelector, c)
}
