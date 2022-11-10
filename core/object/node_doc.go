package object

import (
	"sort"
	"strings"

	"opensvc.com/opensvc/util/key"
	"opensvc.com/opensvc/util/stringslice"
)

func nodeDrvDoc(group, name string) (string, error) {
	buff := ""
	sort.Sort(nodeKeywordStore)
	for _, kw := range nodeKeywordStore {
		if kw.Section != group {
			continue
		}
		if len(kw.Types) > 0 && !stringslice.Has(name, kw.Types) {
			continue
		}
		buff += kw.Doc()
		buff += "\n"
	}
	return buff, nil
}

func nodeSectionDoc(s string) (string, error) {
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

// KeywordDoc returns the documentation of a single keyword.
func (t *Node) KeywordDoc(s string) (string, error) {
	k := key.Parse(s)
	switch {
	case (k.Option != "") && (k.Option != "*"):
		return t.config.Doc(k)
	case k.Section != "":
		return nodeSectionDoc(k.Section)
	default:
		return "", nil
	}
}

// DriverDoc returns the documentation of all keywords of the specified driver.
func (t *Node) DriverDoc(s string) (string, error) {
	l := strings.SplitN(s, ".", 2)
	if len(l) == 2 {
		return nodeDrvDoc(l[0], l[1])
	} else {
		return nodeDrvDoc(s, "")
	}
}
