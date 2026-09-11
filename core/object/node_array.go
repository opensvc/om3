package object

import (
	"strings"

	"github.com/opensvc/om3/v3/core/array"
	"github.com/opensvc/om3/v3/util/key"
)

type ArrayItem struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// Array returns the driver of the array named in the node or cluster
// configuration, or nil when no section names it or no driver serves its type.
//
// The name is the one of the section, and the driver is chosen by the type
// that section declares: the two are not the same, and looking a driver up by
// the name of the array only ever worked for an array named after its own
// type.
func (t *Node) Array(name string) array.Driver {
	if !strings.HasPrefix(name, "array#") {
		name = "array#" + name
	}
	arrayType := t.MergedConfig().Get(key.New(name, "type"))
	if arrayType == "" {
		return nil
	}
	p := array.GetDriver(arrayType)
	if p == nil {
		return nil
	}
	p.SetName(name)
	p.SetConfig(t.MergedConfig())
	return p
}

func (t *Node) Arrays() []array.Driver {
	l := make([]array.Driver, 0)
	for _, item := range t.ListArrays() {
		p := t.Array(item.Name)
		if p == nil {
			continue
		}
		l = append(l, p)
	}
	return l
}

func (t *Node) ListArrays() []ArrayItem {
	l := make([]ArrayItem, 0)
	for _, s := range t.MergedConfig().SectionStrings() {
		if !strings.HasPrefix(s, "array#") {
			continue
		}
		l = append(l, ArrayItem{
			Name: s[6:],
			Type: t.MergedConfig().Get(key.New(s, "type")),
		})
	}
	return l
}
