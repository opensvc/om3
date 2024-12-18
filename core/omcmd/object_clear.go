package omcmd

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
	c, err := client.New()
	if err != nil {
		return err
	}
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	sel := objectselector.New(mergedSelector, objectselector.WithClient(c))
	paths, err := sel.Expand()
	if err != nil {
		return err
	}
	var errs error
	for _, p := range paths {
		nodes, err := nodesFromPaths(c, p.String())
		if err != nil {
			errors.Join(errs, fmt.Errorf("%s: %w", p, err))
			continue
		}
		for _, node := range nodes {
			if resp, err := c.PostInstanceClear(context.Background(), node, p.Namespace, p.Kind, p.Name); err != nil {
				errs = errors.Join(errs, fmt.Errorf("unexpected post object clear %s@%s error %s", p, node, err))
			} else if resp.StatusCode != http.StatusOK {
				errs = errors.Join(errs, fmt.Errorf("unexpected post object clear %s@%s status code %s", p, node, resp.Status))
			}
		}
	}
	return errs
}
