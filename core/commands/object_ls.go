package commands

import (
	"sort"

	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
)

type (
	CmdObjectLs struct {
		OptsGlobal
	}
)

func (t *CmdObjectLs) Run(selector, kind string) error {
	selection := objectselector.NewSelection(
		mergeSelector(selector, t.ObjectSelector, kind, "**"),
		objectselector.SelectionWithLocal(t.Local),
		objectselector.SelectionWithServer(t.Server),
	)
	data := make([]string, 0)
	paths, err := selection.Expand()
	if err != nil {
		return err
	}
	for _, path := range paths {
		data = append(data, path.String())
	}
	sort.Strings(data)
	human := func() string {
		s := ""
		for _, r := range data {
			s += r + "\n"
		}
		return s
	}
	output.Renderer{
		Output:        t.Output,
		Color:         t.Color,
		Data:          data,
		HumanRenderer: human,
		Colorize:      rawconfig.Colorize,
	}.Print()
	return nil
}
