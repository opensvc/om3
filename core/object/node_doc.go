package object

import (
	"sort"
	"strings"

	"opensvc.com/opensvc/util/key"
	"opensvc.com/opensvc/util/stringslice"
)

// Get returns a keyword value
func (t *Node) Doc(options OptsDoc) (string, error) {
	drvDoc := func(group, name string) (string, error) {
		buff := ""
		sort.Sort(nodeKeywordStore)
		for _, kw := range nodeKeywordStore {
			if kw.Section != group {
				continue
			}
			if !stringslice.Has(name, kw.Types) {
				continue
			}
			buff += kw.Doc()
			buff += "\n"
		}
		return buff, nil
	}
	sectionDoc := func(s string) (string, error) {
		buff := ""
		sort.Sort(nodeKeywordStore)
		for _, kw := range nodeKeywordStore {
			if kw.Section != s {
				continue
			}
			buff += kw.Doc()
			buff += "\n"
		}
		return buff, nil
	}

	k := key.Parse(options.Keyword)
	switch {
	case options.Driver != "":
		l := strings.SplitN(options.Driver, ".", 2)
		return drvDoc(l[0], l[1])
	case (k.Option != "") && (k.Option != "*"):
		return t.config.Doc(k)
	case k.Section != "":
		return sectionDoc(k.Section)
	default:
		return "?", nil
	}
	return "", nil
}
