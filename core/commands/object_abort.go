package commands

import (
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/objectselector"
	"opensvc.com/opensvc/util/xerrors"
)

type (
	CmdObjectAbort struct {
		OptsGlobal
	}
)

func (t *CmdObjectAbort) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	sel := objectselector.NewSelection(mergedSelector)
	paths, err := sel.Expand()
	if err != nil {
		return err
	}
	var errs error
	c, err := client.New(client.WithURL(t.Server))
	if err != nil {
		return err
	}
	req := c.NewPostObjectAbort()
	for _, p := range paths {
		req.Path = p
		if _, err := req.Do(); err != nil {
			errs = xerrors.Append(errs, err)
			break // no need to post on every node
		}
	}
	return errs
}
