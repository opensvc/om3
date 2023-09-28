package commands

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/clientcontext"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/core/xconfig"
	"github.com/opensvc/om3/util/editor"
	"github.com/opensvc/om3/util/file"
)

type (
	CmdObjectEditConfig struct {
		OptsGlobal
		Discard bool
		Recover bool
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

func fetchConfig(p path.T, c *client.T) ([]byte, error) {
	resp, err := c.GetObjectFileWithResponse(context.Background(), p.Namespace, p.Kind, p.Name)
	if err != nil {
		return nil, err
	} else if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unexpected get object file status %s", resp.Status())
	}
	return resp.JSON200.Data, nil
}

func pushConfig(p path.T, fName string, c *client.T) (err error) {
	/*
		var cfg *xconfig.T
		if cfg, err = xconfig.NewObject("", fName); err != nil {
			return err
		}
		params := api.PostObjectCreate()
		params.Restore = true
		params.Data[p.String()] = cfg.Raw()
		_, err = c.PostObjectCreate(context.Background(), params)
		if err != nil {
			return err
		}
	*/
	return fmt.Errorf("todo")
}

func (t *CmdObjectEditConfig) doRemote(p path.T, c *client.T) error {
	var (
		err    error
		refSum []byte
		buff   []byte
		f      *os.File
	)
	if buff, err = fetchConfig(p, c); err != nil {
		return err
	}
	if f, err = os.CreateTemp("", ".opensvc.edit.config.*"); err != nil {
		return err
	}
	fName := f.Name()
	defer os.Remove(fName)
	if _, err = f.Write(buff); err != nil {
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
