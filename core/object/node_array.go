package object

import (
	"strings"

	"github.com/opensvc/om3/v3/core/array"
)

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
	for _, name := range t.ListArrays() {
		p := t.Array(name)
		if p == nil {
			continue
		}
		l = append(l, p)
	}
	return l
}

func (t *Node) ListArrays() []string {
	l := make([]string, 0)
	for _, s := range t.MergedConfig().SectionStrings() {
		if !strings.HasPrefix(s, "array#") {
			continue
		}
		l = append(l, s[6:])
	}
	return l
}
