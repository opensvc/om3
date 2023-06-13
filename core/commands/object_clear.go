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
			if resp, err := c.PostObjectClear(context.Background(), params); err != nil {
				errs = errors.Join(errs, fmt.Errorf("unexpected post object clear %s@%s error %s", p, node, err))
			} else if resp.StatusCode != http.StatusOK {
				errs = errors.Join(errs, fmt.Errorf("unexpected post object clear %s@%s status code %s", p, node, resp.Status))
			}
		}
	}
	return errs
}
