package object

import (
	"sort"
	"strings"

	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/naming"
)

// PGKeywords returns the pg_* keywords of the objects of a kind, which set
// the process group caps of the object, of a subset or of a resource, sorted
// by name.
//
// It is what a command setting caps documents and completes, so a keyword
// added to the store is offered by the command without the command changing.
func PGKeywords(kind naming.Kind) []*keywords.Keyword {
	l := make([]*keywords.Keyword, 0)
	for _, kw := range keywordStore.WithKind(kind) {
		if kw.Section != "" || !strings.HasPrefix(kw.Option, "pg_") {
			continue
		}
		l = append(l, kw)
	}
	sort.Slice(l, func(i, j int) bool { return l[i].Option < l[j].Option })
	return l
}
