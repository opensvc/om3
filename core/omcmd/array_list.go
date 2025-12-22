package omcmd

import (
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/util/unstructured"
)

type (
	CmdArrayList struct {
		OptsGlobal
	}
)

func (t *CmdArrayList) Run() error {

	n, err := object.NewNode()
	if err != nil {
		return err
	}
	arrays := n.ListArrays()

	cols := "NAME:name,TYPE:type"

	render := func(items []object.ArrayItem) {
		lines := make(unstructured.List, len(items))
		for i, a := range arrays {
			u := map[string]any{
				"name": a.Name,
				"type": a.Type,
			}
			lines[i] = u
		}
		output.Renderer{
			DefaultOutput: "tab=" + cols,
			Output:        t.Output,
			Color:         t.Color,
			Data:          lines,
			Colorize:      rawconfig.Colorize,
		}.Print()
	}

	render(arrays)

	return err
}
