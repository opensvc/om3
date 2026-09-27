package doc_test

import (
	"reflect"
	"testing"

	"github.com/opensvc/om3/v3/core/doc"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/naming"
)

func TestConvertKeywordStoreProducesStableNonNullCollections(t *testing.T) {
	store := keywords.Store{
		{Section: "z", Option: "last"},
		{Section: "a", Option: "second", Types: []string{"z"}},
		{Section: "a", Option: "first", Kind: naming.NewKinds(naming.KindVol, naming.KindSvc)},
	}

	got := doc.ConvertKeywordStore(store)
	if len(got) != 3 {
		t.Fatalf("got %d items, want 3", len(got))
	}
	if got[0].Section != "a" || got[0].Option != "first" || got[1].Option != "second" || got[2].Section != "z" {
		t.Fatalf("items are not sorted by stable identity: %#v", got)
	}
	for _, item := range got {
		if item.Aliases == nil || item.Candidates == nil || item.Depends == nil || item.Kind == nil || item.Types == nil {
			t.Fatalf("item contains a nil collection: %#v", item)
		}
	}
	if !reflect.DeepEqual(got[0].Kind, []string{"svc", "vol"}) {
		t.Fatalf("got sorted kinds %#v, want [svc vol]", got[0].Kind)
	}
}
