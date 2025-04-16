package oxcmd

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/editor"
	"github.com/opensvc/om3/util/file"
)

type (
	CmdObjectKeyEdit struct {
		OptsGlobal
		Name string
	}
)

func (t *CmdObjectKeyEdit) do(selector string, c *client.T) error {
	c, err := client.New()
	if err != nil {
		return err
	}
	sel := objectselector.New(selector, objectselector.WithClient(c))
	paths, err := sel.MustExpand()
	if err != nil {
		return err
	}
	for _, p := range paths {
		if err := t.DoRemote(p, c); err != nil {
			return err
		}
	}
	return nil
}

func fetchKey(p naming.Path, key string, c *client.T) (s []byte, err error) {
	params := api.GetObjectDataKeyParams{
		Name: key,
	}
	resp, err := c.GetObjectDataKeyWithResponse(context.Background(), p.Namespace, p.Kind, p.Name, &params)
	if err != nil {
		return []byte{}, err
	}
	if resp.StatusCode() == http.StatusOK {
		return resp.Body, nil
	}
	return []byte{}, fmt.Errorf("%s", resp.Status())
}

func pushKey(p naming.Path, key string, fName string, c *client.T) (err error) {
	r, err := os.Open(fName)
	if err != nil {
		return err
	}
	defer r.Close()
	params := api.PutObjectDataKeyParams{
		Name: key,
	}
	resp, err := c.PutObjectDataKeyWithBody(context.Background(), p.Namespace, p.Kind, p.Name, &params, "application/octet-stream", r)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("%s", resp.Status)
	}
	return nil
}

func (t *CmdObjectKeyEdit) DoRemote(p naming.Path, c *client.T) error {
	var (
		err    error
		refSum []byte
		buff   []byte
		f      *os.File
	)
	if buff, err = fetchKey(p, t.Name, c); err != nil {
		return err
	}
	if f, err = os.CreateTemp("", ".opensvc.edit.key.*"); err != nil {
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
	if err = pushKey(p, t.Name, fName, c); err != nil {
		return err
	}
	return nil
}

func (t *CmdObjectKeyEdit) Run(selector, kind string) error {
	var (
		c   *client.T
		err error
	)
	mergedSelector := commoncmd.MergeSelector(selector, t.ObjectSelector, kind, "")
	if c, err = client.New(); err != nil {
		return err
	}
	if err = t.do(mergedSelector, c); err != nil {
		return err
	}
	return nil
}
