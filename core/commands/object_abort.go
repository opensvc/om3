package commands

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/daemon/api"
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
		if resp, err := c.PostObjectAbort(context.Background(), params); err != nil {
			errs = errors.Join(errs, err)
		} else if resp.StatusCode != http.StatusOK {
			errs = errors.Join(errs, fmt.Errorf("unexpected post object abort status code %s", resp.Status))
		}
	}
	return errs
}
