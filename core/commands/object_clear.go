package commands

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/objectselector"
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
		nodes, err := nodesFromPath(p)
		if err != nil {
			errors.Join(errs, fmt.Errorf("%s: %w", p, err))
			continue
		}
		for _, node := range nodes {
			c, err := client.New(
				client.WithURL(node),
			)
			if err != nil {
				return err
			}
			if resp, err := c.PostInstanceClear(context.Background(), p.Namespace, p.Kind.String(), p.Name); err != nil {
				errs = errors.Join(errs, fmt.Errorf("unexpected post object clear %s@%s error %s", p, node, err))
			} else if resp.StatusCode != http.StatusOK {
				errs = errors.Join(errs, fmt.Errorf("unexpected post object clear %s@%s status code %s", p, node, resp.Status))
			}
		}
	}
	return errs
}
