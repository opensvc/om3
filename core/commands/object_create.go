package commands

import (
	"fmt"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/entrypoints/create"
	"opensvc.com/opensvc/core/objectselector"
	"opensvc.com/opensvc/core/path"
)

type (
	CmdObjectCreate struct {
		OptsGlobal
		OptsLock
		OptsResourceSelector
		OptTo
		Template    string
		Config      string
		Keywords    []string
		Env         string
		Interactive bool
		Provision   bool
		Restore     bool
		Force       bool
		Namespace   string
	}
)

func (t *CmdObjectCreate) Run(selector, kind string) error {
	p, err := t.parseSelector(selector, kind)
	if err != nil {
		return err
	}
	c, err := client.New(client.WithURL(t.Server))
	if err != nil {
		return err
	}
	cr, err := create.New(
		create.WithClient(c),
		create.WithPath(p),
		create.WithNamespace(t.Namespace),
		create.WithTemplate(t.Template),
		create.WithConfig(t.Config),
		create.WithKeywords(t.Keywords),
		create.WithRestore(t.Restore),
		create.WithForce(t.Force),
	)
	if err != nil {
		return err
	}
	return cr.Do()
}

func (t *CmdObjectCreate) parseSelector(selector, kind string) (path.T, error) {
	if selector == "" {
		// allowed with multi-definitions fed via stdin
		return path.T{}, nil
	}
	p, err := path.Parse(selector)
	if err != nil {
		return p, err
	}
	// now we know the path is valid. Verify it is non-existing or matches only one object.
	objectSelector := mergeSelector(selector, t.ObjectSelector, kind, "**")
	paths, err := objectselector.NewSelection(
		objectSelector,
		objectselector.SelectionWithLocal(t.Local),
		objectselector.SelectionWithServer(t.Server),
	).Expand()
	if err == nil && len(paths) > 1 {
		return p, fmt.Errorf("at most one object can be selected for create. to create many objects in a single create, use --config - and pipe json definitions.")
	}
	return p, nil
}
