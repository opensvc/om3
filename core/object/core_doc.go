package object

import (
	"fmt"
	"sort"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/util/key"
)

func drvDoc(drvID driver.ID, kwName string) (string, error) {
	factory := resource.NewResourceFunc(drvID)
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

func (t core) defaultDoc() (string, error) {
	buff := ""
	sort.Sort(keywordStore)
	for _, kw := range keywordStore {
		if kw.Section != "DEFAULT" {
			continue
		}
		if !kw.Kind.Has(t.path.Kind) {
			continue
		}
		buff += kw.Doc()
		buff += "\n"
	}
	return buff, nil
}

// KeywordDoc returns the documentation of a single keyword.
func (t *core) KeywordDoc(s string) (string, error) {
	k := key.Parse(s)
	switch {
	case k.Option != "":
		return t.config.Doc(k)
	case k.Section == "DEFAULT":
		return t.defaultDoc()
	case k.Section != "":
		drvID, _ := driverIDFromRID(t, k.Section)
		return drvDoc(drvID, "")
	default:
		return "", nil
	}
}

// DriverDoc returns the documentation of all keywords of the specified driver.
func (t *core) DriverDoc(s string) (string, error) {
	drvID := driver.Parse(s)
	return drvDoc(drvID, s)
}
