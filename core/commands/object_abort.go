package commands

import (
	"context"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/xerrors"
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
	params := api.PostObjectAbort{}
	for _, p := range paths {
		params.Path = p.String()
		if _, err := c.PostObjectAbort(context.Background(), params); err != nil {
			errs = xerrors.Append(errs, err)
			break // no need to post on every node
		}
	}
	return errs
}
