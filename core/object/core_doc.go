package object

import (
	"fmt"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/util/key"
)

func KeywordStoreWithDrivers(kind naming.Kind) keywords.Store {
	var store keywords.Store
	if kind == naming.KindCcfg {
		store = append(store, ccfgKeywordStore...)
	} else {
		store = append(store, keywordStore...)
	}
	for _, drvID := range driver.List() {
		factory := resource.NewResourceFunc(drvID)
		if factory == nil {
			// node drivers don't have a factory, skip them
			continue
		}
		r := factory()
		manifest := r.Manifest()
		if !manifest.Kinds.Has(kind) {
			continue
		}
		for _, kw := range manifest.Keywords() {
			kw.Section = drvID.Group.String()
			kw.Types = []string{drvID.Name}
			store = append(store, kw)
		}
	}
	return store
}

func (t *core) Doc(drvStr, kwStr string, depth int) (string, error) {
	store := KeywordStoreWithDrivers(t.path.Kind)
	switch {
	case drvStr == "" && kwStr == "":
		return store.Doc(t.path.Kind, depth)
	case drvStr != "" && kwStr != "":
		l := keywords.ParseIndex(drvStr)
		return store.KeywordDoc(l[0], l[1], kwStr, t.path.Kind, depth)
	case drvStr != "":
		l := keywords.ParseIndex(drvStr)
		return store.DriverDoc(l[0], l[1], t.path.Kind, depth)
	case kwStr != "":
		k := key.Parse(kwStr)
		sectionType := t.config.SectionType(k)
		kw := t.config.Referrer.KeywordLookup(k, sectionType)
		if kw.IsZero() {
			return "", fmt.Errorf("keyword not found")
		}
		return kw.Doc(depth), nil
	default:
		return "", nil
	}
}
