package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/entrypoints/create"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
)

type (
	// CmdObjectCreate is the cobra flag set of the create command.
	CmdObjectCreate struct {
		object.OptsCreate
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectCreate) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *CmdObjectCreate) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:   "create",
		Short: "create new objects",
		Run: func(cmd *cobra.Command, args []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdObjectCreate) run(selector *string, kind string) {
	if err := t.runErr(selector, kind); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func (t *CmdObjectCreate) runErr(selector *string, kind string) error {
	p, err := t.parseSelector(selector, kind)
	if err != nil {
		return err
	}
	c, err := client.New(client.WithURL(t.Global.Server))
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
	)
	if err != nil {
		return err
	}
	return cr.Do()
}

func (t *CmdObjectCreate) parseSelector(selector *string, kind string) (path.T, error) {
	if *selector == "" {
		// allowed with multi-definitions fed via stdin
		return path.T{}, nil
	}
	p, err := path.Parse(*selector)
	if err != nil {
		return p, err
	}
	// now we know the path is valid. Verify it is non-existing or matches only one object.
	objectSelector := mergeSelector(*selector, t.Global.ObjectSelector, kind, "**")
	paths, err := object.NewSelection(
		objectSelector,
		object.SelectionWithLocal(t.Global.Local),
		object.SelectionWithServer(t.Global.Server),
	).Expand()
	if err == nil && len(paths) > 1 {
		return p, fmt.Errorf("at most one object can be selected for create. to create many objects in a single create, use --config - and pipe json definitions.")
	}
	return p, nil
}
