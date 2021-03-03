package entrypoints

import (
	"sort"

	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/output"
)

// List is the struct exposing the object selection printing method.
type List struct {
	ObjectSelector string
	Format         string
	Color          string
}

// Do prints the formatted object selection
func (t List) Do() {
	results := object.NewSelection(t.ObjectSelector).Action("List")
	data := make([]string, 0)
	for _, r := range results {
		buff, ok := r.Data.(string)
		if !ok {
			continue
		}
		data = append(data, buff)
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
