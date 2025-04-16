package oxcmd

import (
	"fmt"
	"os"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/util/editor"
	"github.com/opensvc/om3/util/file"
)

type (
	CmdObjectConfigEdit struct {
		OptsGlobal
		Discard bool
		Recover bool
	}
)

func (t *CmdObjectConfigEdit) do(selector string, c *client.T) error {
	sel := objectselector.New(
		selector,
		objectselector.WithClient(c),
	)
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

func (t *CmdObjectConfigEdit) DoRemote(p naming.Path, c *client.T) error {
	var (
		err      error
		refSum   []byte
		filename string
	)
	if filename, err = createTempRemoteObjectConfig(p, c); err != nil {
		return err
	}
	defer os.Remove(filename)
	if refSum, err = file.MD5(filename); err != nil {
		return err
	}
	if err = editor.Edit(filename); err != nil {
		return err
	}
	if file.HaveSameMD5(refSum, filename) {
		fmt.Println("unchanged")
		return nil
	}
	if err = putObjectConfig(p, filename, c); err != nil {
		return err
	}
	return nil
}

func (t *CmdObjectConfigEdit) Run(selector, kind string) error {
	var (
		c   *client.T
		err error
	)
	mergedSelector := commoncmd.MergeSelector(selector, t.ObjectSelector, kind, "")
	if c, err = client.New(); err != nil {
		return err
	}
	return t.do(mergedSelector, c)
}
