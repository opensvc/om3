package commands

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/clientcontext"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/util/editor"
	"github.com/opensvc/om3/util/file"
)

type (
	CmdObjectEditKey struct {
		OptsGlobal
		Key string
	}
)

func (t *CmdObjectEditKey) do(selector string, c *client.T) error {
	sel := objectselector.NewSelection(selector)
	wc := clientcontext.IsSet()
	paths, err := sel.Expand()
	if err != nil {
		return err
	}
	for _, p := range paths {
		obj, err := object.New(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skip %s: %s\n", p, err)
			continue
		}
		var (
			ks object.Keystore
			ok bool
		)
		if ks, ok = obj.(object.Keystore); !ok {
			fmt.Fprintf(os.Stderr, "skip %s: not a keystore\n", p)
			continue
		}
		if !wc && p.Exists() {
			if err := t.doLocal(ks, c); err != nil {
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

func (t *CmdObjectEditKey) doLocal(obj object.Keystore, c *client.T) error {
	return obj.EditKey(t.Key)
}

func fetchKey(p path.T, key string, c *client.T) (s []byte, err error) {
	var (
		b []byte
	)
	handle := c.NewGetKey()
	handle.Path = p.String()
	handle.Key = key
	b, err = handle.Do()
	if err != nil {
		return []byte{}, err
	}
	resp := struct {
		data []byte
	}{}
	if err = json.Unmarshal(b, &resp); err != nil {
		return []byte{}, err
	}
	return resp.data, nil
}

func pushKey(p path.T, key string, fName string, c *client.T) (err error) {
	var b []byte
	if b, err = os.ReadFile(fName); err != nil {
		return err
	}
	req := c.NewPostKey()
	req.Path = p.String()
	req.Key = key
	req.Data = b
	_, err = req.Do()
	if err != nil {
		return err
	}
	return nil
}

func (t *CmdObjectEditKey) doRemote(p path.T, c *client.T) error {
	var (
		err    error
		refSum []byte
		buff   []byte
		f      *os.File
	)
	if buff, err = fetchKey(p, t.Key, c); err != nil {
		return err
	}
	if f, err = ioutil.TempFile("", ".opensvc.edit.key.*"); err != nil {
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
	if err = pushKey(p, t.Key, fName, c); err != nil {
		return err
	}
	return nil
}

func (t *CmdObjectEditKey) Run(selector, kind string) error {
	var (
		c   *client.T
		err error
	)
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	if c, err = client.New(client.WithURL(t.Server)); err != nil {
		return err
	}
	if err = t.do(mergedSelector, c); err != nil {
		return err
	}
	return nil
}
