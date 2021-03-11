package entrypoints

import (
	"sort"

	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/output"
)

// List is the struct exposing the object selection printing method.
type List struct {
	ObjectSelector string
	Color          string
	Format         string
	Server         string
	Local          bool
}

// Do prints the formatted object selection
func (t List) Do() {
	selection := object.NewSelection(t.ObjectSelector)
	selection.SetLocal(t.Local)
	selection.SetServer(t.Server)
	data := make([]string, 0)
	for _, path := range selection.Expand() {
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
		Format:        t.Format,
		Color:         t.Color,
		Data:          data,
		HumanRenderer: human,
	}.Print()
}
