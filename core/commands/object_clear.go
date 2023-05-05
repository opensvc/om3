package commands

import (
	"context"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/xerrors"
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
			params := api.PostObjectClear{
				Path: p.String(),
			}
			if _, err := c.PostObjectClear(context.Background(), params); err != nil {
				errs = xerrors.Append(errs, err)
			}
		}
	}
	return errs
}
