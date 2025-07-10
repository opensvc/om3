package object

import (
	"fmt"

	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/util/key"
)

func (t *Node) Doc(drvStr, kwStr string, depth int) (string, error) {
	store := NodeKeywordStore
	switch {
	case drvStr == "" && kwStr == "":
		return store.Doc(naming.KindInvalid, depth)
	case drvStr != "" && kwStr != "":
		l := keywords.ParseIndex(drvStr)
		return store.KeywordDoc(l[0], l[1], kwStr, naming.KindInvalid, depth)
	case drvStr != "":
		l := keywords.ParseIndex(drvStr)
		return store.DriverDoc(l[0], l[1], naming.KindInvalid, depth)
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
