package object

import (
	"strings"

	"github.com/opensvc/om3/v3/core/array"
	"github.com/opensvc/om3/v3/util/key"
)

type ArrayItem struct {
	Name string
	Type string
}

func (t *Node) Array(name string) array.Driver {
	p := array.GetDriver(name)
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
