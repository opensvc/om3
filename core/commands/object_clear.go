package commands

import (
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/objectselector"
	"opensvc.com/opensvc/util/xerrors"
)

type (
	CmdObjectClear struct {
		OptsGlobal
	}
)

func (t *CmdObjectClear) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	sel := objectselector.NewSelection(mergedSelector)
	paths, err := sel.Expand()
	if err != nil {
		return err
	}
	var errs error
	for _, p := range paths {
		for _, node := range nodesFromPath(p) {
			c, err := client.New(
				client.WithURL(node),
			)
			if err != nil {
				return err
			}
			req := c.NewPostObjectClear()
			req.Path = p
			if _, err := req.Do(); err != nil {
				errs = xerrors.Append(errs, err)
			}
		}
	}
	return errs
}
