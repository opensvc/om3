package omcmd

import (
	"fmt"
	"os"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/clientcontext"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectselector"
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
	sel := objectselector.New(
		selector,
		objectselector.WithClient(c),
	)
	paths, err := sel.MustExpand()
	if err != nil {
		return err
	}
	wc := clientcontext.IsSet()
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
	if err != nil {
		return err
	}
	return nil
}

func (t *CmdObjectEditConfig) doRemote(p naming.Path, c *client.T) error {
	var (
		err      error
		refSum   []byte
		filename string
	)
	if filename, err = createTempRemoteConfig(p, c); err != nil {
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
	if err = putConfig(p, filename, c); err != nil {
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
	if c, err = client.New(); err != nil {
		return err
	}
	return t.do(mergedSelector, c)
}
