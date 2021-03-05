package entrypoints

import (
	"sort"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/output"
)

// List is the struct exposing the object selection printing method.
type List struct {
	ObjectSelector string
	Color          string
	Format         string
	Server         string
}

// Do prints the formatted object selection
func (t List) Do() {
	c := client.NewConfig()
	c.SetURL(t.Server)
	api, _ := c.NewAPI()
	selection := object.NewSelection(t.ObjectSelector)
	selection.SetAPI(api)
	results := selection.Action("List")
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
