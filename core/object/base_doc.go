package object

import (
	"fmt"
	"sort"

	"opensvc.com/opensvc/core/driver"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/util/key"
)

// OptsGet is the options of the Get function of all base objects.
type OptsDoc struct {
	Global  OptsGlobal
	Keyword string `flag:"kw"`
	Driver  string `flag:"driver"`
}

// Get returns a keyword value
func (t *Base) Doc(options OptsDoc) (string, error) {
	drvDoc := func(did driver.ID, kwName string) (string, error) {
		factory := resource.NewResourceFunc(did)
		if factory == nil {
			return "", fmt.Errorf("driver not found")
		}
		r := factory()
		buff := ""
		store := keywords.Store(r.Manifest().Keywords)
		sort.Sort(store)
		for _, kw := range store {
			if (kwName != "") && kw.Option != kwName {
				continue
			}
			buff += kw.Doc()
			buff += "\n"
		}
		return buff, nil
	}
	defaultDoc := func() (string, error) {
		buff := ""
		sort.Sort(keywordStore)
		for _, kw := range keywordStore {
			if kw.Section != "DEFAULT" {
				continue
			}
			if !kw.Kind.Has(t.Path.Kind) {
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
		did := driver.Parse(options.Driver)
		return drvDoc(*did, options.Keyword)
	case k.Option != "":
		return t.config.Doc(k)
	case k.Section == "DEFAULT":
		return defaultDoc()
	case k.Section != "":
		did, _ := driverIDFromRID(t, k.Section)
		return drvDoc(did, "")
	default:
		return "?", nil
	}
	return "", nil
}
